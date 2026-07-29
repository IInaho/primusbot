package app

import (
	"context"
	"os"
	"sync"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/command"
	"nekocode/bot/config"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/extension"
	"nekocode/bot/policy"
	"nekocode/bot/policy/builtin"
	systemprompt "nekocode/bot/prompt/system"
	"nekocode/bot/provider"
	"nekocode/bot/session"
	"nekocode/bot/tools/builtin/catalog"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/workspace"
	"nekocode/bot/view"
	commonview "nekocode/common/view"
)

type Bot struct {
	cwd            string
	cfg            *config.Config
	promptBuilder  *systemprompt.Builder
	ctxMgr         *ctxmgr.Manager
	policy         *policy.Policy
	ag             *runtime.Agent
	toolbox        *catalog.Toolbox
	cmd            *command.Handler
	ext            *extension.Manager
	cb             *callbackBus
	sess           *session.Manager
	sessionResumed bool
	mu             sync.Mutex
}

func New() *Bot {
	b := &Bot{}
	b.cwd, _ = os.Getwd()

	b.initConfig()
	b.initCtxMgr()
	b.initSession()
	b.rebuildRuntime()

	return b
}

// Close releases lifecycle resources held by the bot (currently the
// stateful shell tool inside the toolbox).
func (b *Bot) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ext != nil {
		b.ext.Close()
	}
	b.toolbox.Shutdown()
}

func (b *Bot) initConfig() {
	b.cfg, _ = config.Load()
	b.promptBuilder = systemprompt.NewBuilder(b.cwd)
}

func (b *Bot) initCtxMgr() {
	systemPrompt := b.promptBuilder.Build()
	memFile, _ := memory.Load(memory.DefaultPath())
	b.ctxMgr = ctxmgr.New(ctxmgr.Config{SystemPrompt: systemPrompt, Memory: memFile})
	if b.cfg != nil && b.cfg.ContextWindow > 0 {
		b.ctxMgr.SetContextWindow(b.cfg.ContextWindow)
	}
}

// rebuildRuntime recreates components affected by configuration changes.
// Keep this order: tools and policy are dependencies of extensions and agent;
// commands consume all of them.
func (b *Bot) rebuildRuntime() {
	if b.ext != nil {
		b.ext.Close()
	}
	b.initToolRegistry()
	b.initPolicy()

	if b.cb == nil {
		b.cb = &callbackBus{}
	}
	b.cb.cwd = b.cwd
	b.cb.home, _ = os.UserHomeDir()
	if b.cfg != nil {
		b.cb.policyCfg = b.cfg.Permissions
	}

	os.Setenv("NEKOCODE_WORKSPACE", b.cwd)
	workspace.Configure(b.cwd, b.configuredWorkspaceRoots())
	if b.ctxMgr != nil && b.cfg != nil && b.cfg.ContextWindow > 0 {
		b.ctxMgr.SetContextWindow(b.cfg.ContextWindow)
	}

	b.initExtensions()
	b.initAgent()
	b.ctxMgr.SetSummarizer(ctxmgr.MakeSummarizer(context.Background(), b.ctxMgr.MergeClient()))
	b.initCommands()
}

func (b *Bot) configuredWorkspaceRoots() []workspace.Root {
	var roots []workspace.Root
	if b.cfg != nil {
		for _, r := range b.cfg.Workspaces {
			access := workspace.Access(r.Access)
			if access == "" {
				access = workspace.AccessReadOnly
			}
			roots = append(roots, workspace.Root{Path: r.Path, Access: access})
		}
	}
	for _, r := range permission.NewStore(b.cwd).WorkspaceRoots() {
		roots = append(roots, workspace.Root{Path: r.Path, Access: workspace.Access(r.Access)})
	}
	return roots
}

func (b *Bot) initToolRegistry() {
	if b.toolbox == nil {
		b.toolbox = catalog.NewToolbox(b.cfg.ImageGenModels)
		return
	}
	b.toolbox.RebuildRegistry(b.cfg.ImageGenModels)
}

func (b *Bot) initPolicy() {
	b.policy = policy.New()
	if b.sess != nil {
		b.policy.SetSessionID(b.sess.CurrentID())
	}
	builtin.Register(b.policy)
}

func (b *Bot) initAgent() {
	am := b.cfg.ActiveModelConfig()
	llmClient := provider.NewClientWithProtocol(am.APIKey, am.BaseURL, am.Model, am.Protocol)

	fm := b.cfg.ResolveModel(b.cfg.FlashModel)
	mergeClient := provider.NewClientWithProtocol(fm.APIKey, fm.BaseURL, fm.Model, fm.Protocol)
	mergeClient.SetDisableThinking(true)
	mergeClient.SetMaxTokens(2000)
	b.ctxMgr.SetMergeClient(mergeClient)

	b.ag = runtime.New(context.Background(), runtime.AgentConfig{
		CtxMgr:   b.ctxMgr,
		LLM:      llmClient,
		Registry: b.toolbox.Registry,
		Policy:   b.policy,
		TodoWriter: func(items []commonview.TodoItem) {
			b.ctxMgr.SetTodos(items)
			if b.cb.todoFn != nil {
				b.cb.todoFn(items)
			}
		},
	})
	b.applyCallbacks()

	// Inject the permission engine into the shell tool so builtin sandbox
	// rules (e.g. pnpm dev → network) are applied.
	b.toolbox.SetSandboxProfiler(b.ag.SandboxProfiler())

	b.wireTaskTool(fm, b.ag)
}

func (b *Bot) applyCallbacks() {
	b.cb.applyTo(b.ag)
	b.setQuestionFunc(b.cb.questionFn)
}

func (b *Bot) setQuestionFunc(fn view.QuestionFunc) {
	if fn == nil || b.toolbox.Registry == nil {
		return
	}
	t, err := b.toolbox.Registry.Get("question")
	if err != nil {
		return
	}
	if qt, ok := t.(interface{ SetQuestionFunc(view.QuestionFunc) }); ok {
		qt.SetQuestionFunc(fn)
	}
}

func (b *Bot) initCommands() {
	deps := command.Deps{
		CtxMgr:       b.ctxMgr,
		Ag:           func() command.PlanModeController { return b.getAgent().Executor() },
		Skills:       b.ext,
		ToolRegistry: b.toolbox.Registry,
		Policy:       b.policy,
		GetConfigFn:  b.ProviderModel,
		ListModelsFn: b.cfg.AllModelNames,
		SwitchModel:  b.SwitchModel,
	}
	if b.cmd == nil {
		b.cmd = command.NewHandler(deps)
	} else {
		b.cmd.RegisterAll(deps)
	}

	b.ext.RegisterCommands(b.cmd.Parser(), b.cb.installCallbacks())
	b.registerSessionCommands(b.cmd.Parser())
}
