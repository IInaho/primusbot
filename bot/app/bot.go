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
	"nekocode/bot/policy"
	"nekocode/bot/policy/builtin"
	systemprompt "nekocode/bot/prompt/system"
	"nekocode/bot/provider"
	"nekocode/bot/tools/builtin/catalog"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/workspace"
	"nekocode/bot/view"
	commonview "nekocode/common/view"
)

type Bot struct {
	cwd           string
	cfg           *config.Config
	promptBuilder *systemprompt.Builder
	ctxMgr        *ctxmgr.Manager
	hookReg       *policy.Registry
	ag            *runtime.Agent
	toolbox       *catalog.Toolbox
	cmd           *command.Handler
	ext           *extensionFacade
	cb            *callbackBus
	subWiring     *subagentWiring
	sess          *sessionFacade
	mu            sync.Mutex
}

func New() *Bot {
	b := &Bot{}
	b.cwd, _ = os.Getwd()

	b.initConfig()
	b.initCtxMgr()
	b.initSession()
	b.reinit()

	return b
}

// Close releases lifecycle resources held by the bot (currently the
// stateful shell tool inside the toolbox).
func (b *Bot) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
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

// reinit rebuilds the runtime facades, agent, summarizer, and commands.
// Called from New() for initial setup and from ApplyConfig() for hot reload.
//
// 步骤顺序由隐式依赖决定（见各步注释），调整前先确认依赖方。
func (b *Bot) reinit() {
	// 1. 工具环境最先：ext/subWiring 构造要拿 Registry，initAgent 的
	//    AgentConfig.Registry 也是它。
	b.initToolRegistry()
	// 2. 钩子注册表其次：ext 构造和 initAgent 的 AgentConfig.HookReg 都消费它。
	b.initHooks()
	if b.cb == nil {
		b.cb = &callbackBus{}
	}
	// Inject the declarative permission policy (from config) and workspace/home
	// so the rule engine can resolve path anchors. cfg may be nil if Load failed.
	// 必须在 initAgent 之前：applyCallbacks 把 policy/confirm 灌进 executor。
	b.cb.cwd = b.cwd
	b.cb.home, _ = os.UserHomeDir()
	if b.cfg != nil {
		b.cb.policyCfg = b.cfg.Permissions
	}
	// Expose the workspace to toolutil so write/edit enforce path boundaries
	// against the session workspace rather than the process cwd (which differs
	// in tests and may differ after /cd).
	os.Setenv("NEKOCODE_WORKSPACE", b.cwd)
	workspace.Configure(b.cwd, b.configuredWorkspaceRoots())
	if b.ctxMgr != nil && b.cfg != nil && b.cfg.ContextWindow > 0 {
		b.ctxMgr.SetContextWindow(b.cfg.ContextWindow)
	}
	// 3. 扩展与子代理门面：依赖 toolbox.Registry 和 hookReg（第 1、2 步）。
	b.ext = newExtensionFacade(b.ctxMgr, b.toolbox.Registry, b.hookReg, b.cfg.ContextWindow)
	b.subWiring = newSubagentWiring(b.toolbox.Registry, b.ctxMgr, b.cwd, b.cfg.ContextWindow)
	// 4. 扩展初始化：向 Registry 注册插件/MCP 工具、加载技能；
	//    先于 initCommands（$skill 命令来自 ext.skills）。
	b.ext.InitPlugins()
	b.ext.InitConfigMCPServers(b.cfg.MCPServers)
	b.ext.InitSkills()
	// 5. Agent：依赖第 1-3 步的全部产物；applyCallbacks 还会向 Registry
	//    里的 question 工具写回调。同时把 merge client 写进 ctxMgr。
	b.initAgent()
	// 6. Summarizer 在 agent 之后：它读 ctxMgr.MergeClient()，
	//    而 merge client 是 initAgent 刚设置的。
	b.initSummarizer()
	// 7. 命令最后：Deps 同时引用 agent（/plan）、ext.skills（$skill）、
	//    ctxMgr 与 Registry。
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

func (b *Bot) initSummarizer() {
	b.ctxMgr.SetSummarizer(ctxmgr.MakeSummarizer(context.Background(), b.ctxMgr.MergeClient()))
}

func (b *Bot) initToolRegistry() {
	if b.toolbox == nil {
		b.toolbox = catalog.NewToolbox(b.cfg.ImageGenModels)
		return
	}
	b.toolbox.RebuildRegistry(b.cfg.ImageGenModels)
}

func (b *Bot) initHooks() {
	b.hookReg = policy.NewRegistry()
	if b.sess != nil {
		b.hookReg.SetSessionID(b.sess.CurrentID())
	}
	builtin.Register(b.hookReg)
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
		HookReg:  b.hookReg,
		TodoWriter: func(items []commonview.TodoItem) {
			b.ctxMgr.SetTodos(items)
			b.cb.todoWriter()(todoItemsToView(items))
		},
	})
	b.applyCallbacks()

	// Inject the permission engine into the shell tool so builtin sandbox
	// rules (e.g. pnpm dev → network) are applied.
	b.toolbox.SetSandboxProfiler(b.ag.SandboxProfiler())

	b.subWiring.WireTaskTool(fm, b.ag)
}

func (b *Bot) applyCallbacks() {
	b.cb.applyAgentControlCallbacksTo(b.ag)
	b.setQuestionFunc(b.cb.questionFn)
}

func todoItemsToView(items []commonview.TodoItem) []view.TodoItem {
	out := make([]view.TodoItem, 0, len(items))
	for _, item := range items {
		out = append(out, view.TodoItem{Content: item.Content, Status: item.Status})
	}
	return out
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
	skills := skillCommandProvider{manager: b.ext.skills}
	deps := command.Deps{
		CtxMgr:       b.ctxMgr,
		Ag:           func() command.PlanModeController { return b.getAgent().Executor() },
		Skills:       skills,
		ToolRegistry: b.toolbox.Registry,
		HookReg:      b.hookReg,
		GetConfigFn:  b.ProviderModel,
		ListModelsFn: b.cfg.AllModelNames,
		SwitchModel:  b.SwitchModel,
	}
	if b.cmd == nil {
		b.cmd = command.NewHandler(deps)
	} else {
		b.cmd.RegisterAll(deps)
	}

	b.ext.RegisterPluginCommands(b.cmd.Parser(), b.cb.InstallCallbacks())
	b.sess.RegisterCommands(b.cmd.Parser())
}
