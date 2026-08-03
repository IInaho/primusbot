// Package bot assembles NekoCode's transport-neutral agent foundation.
package bot

import (
	"context"
	"fmt"
	"os"
	"sync"

	"nekocode/bot/agent"
	"nekocode/bot/command"
	"nekocode/bot/config"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/extension"
	"nekocode/bot/policy"
	"nekocode/bot/policy/builtin"
	"nekocode/bot/prompt"
	"nekocode/bot/provider"
	"nekocode/bot/session"
	"nekocode/bot/tools/builtin/catalog"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/workspace"
	"nekocode/protocol"
)

// RunHost is the synchronous boundary used by one bot run. Runtime and other
// application layers adapt this port without leaking their view models into
// the bot.
type RunHost interface {
	Text(string)
	Reason(string)
	Step(protocol.StepEvent)
	Phase(string)
	Todos([]protocol.TodoItem)
	Confirm(protocol.ConfirmRequest) protocol.ConfirmReply
	Ask(protocol.QuestionRequest) protocol.QuestionReply
}

type Bot struct {
	cwd           string
	home          string
	cfg           *config.Config
	promptBuilder *prompt.Builder
	ctxMgr        *ctxmgr.Manager
	policy        *policy.Policy
	ag            *agent.Agent
	toolbox       *catalog.Toolbox
	cmd           *command.Handler
	ext           *extension.Manager
	sess          *session.Manager
	mu            sync.Mutex
	hostMu        sync.RWMutex
	runHost       RunHost
}

// New assembles the standard bot and loads its persisted configuration,
// memory, and current session.
func New() (*Bot, error) {
	b := &Bot{}
	var err error
	if b.cwd, err = os.Getwd(); err != nil {
		return nil, fmt.Errorf("bot: resolve working directory: %w", err)
	}
	if b.home, err = os.UserHomeDir(); err != nil {
		return nil, fmt.Errorf("bot: resolve home directory: %w", err)
	}
	if err := b.initConfig(); err != nil {
		return nil, err
	}
	if err := b.initCtxMgr(); err != nil {
		return nil, err
	}
	b.initSession()
	if err := b.rebuildRuntime(); err != nil {
		return nil, err
	}
	return b, nil
}

// Close releases lifecycle resources held by the bot (currently the
// stateful shell tool inside the toolbox).
func (b *Bot) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ext != nil {
		b.ext.Close()
	}
	if b.toolbox == nil {
		return nil
	}
	return b.toolbox.Close()
}

func (b *Bot) initConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("bot: load config: %w", err)
	}
	b.cfg = cfg
	b.promptBuilder = prompt.New(b.cwd)
	b.promptBuilder.SetEnvironmentProvider(b.environment)
	return nil
}

func (b *Bot) initCtxMgr() error {
	systemPrompt := b.promptBuilder.BuildStatic()
	memFile, err := memory.Load(memory.DefaultPath())
	if err != nil {
		return fmt.Errorf("bot: load memory: %w", err)
	}
	b.ctxMgr = ctxmgr.New(ctxmgr.Config{
		SystemPrompt:  systemPrompt,
		ContextWindow: b.cfg.EffectiveContextWindow(),
		Memory:        memFile,
	})
	b.ctxMgr.SetRuntimePromptProvider(b.promptBuilder.BuildEnvironment)
	return nil
}

// rebuildRuntime recreates components affected by configuration changes.
// Keep this order: tools and policy are dependencies of extensions and agent;
// commands consume all of them.
func (b *Bot) rebuildRuntime() error {
	if b.ext != nil {
		b.ext.Close()
	}
	b.initToolRegistry()
	b.initPolicy()

	b.toolbox.Workspace().Configure(b.cwd, b.configuredWorkspaceRoots())
	if b.ctxMgr != nil && b.cfg != nil {
		b.ctxMgr.SetContextWindow(b.cfg.EffectiveContextWindow())
	}

	b.initExtensions()
	b.initAgent()
	b.initCommands()
	return nil
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
	b.syncPolicySessionID()
	builtin.Register(b.policy)
}

func (b *Bot) initAgent() {
	am := b.cfg.ActiveModelConfig()
	llmClient := provider.New(provider.Config{
		APIKey: am.APIKey, BaseURL: am.BaseURL, Model: am.Model, Protocol: am.Protocol,
	})

	fm := b.cfg.ResolveModel(b.cfg.FlashModel)
	compactionModel := provider.New(provider.Config{
		APIKey: fm.APIKey, BaseURL: fm.BaseURL, Model: fm.Model, Protocol: fm.Protocol,
	})
	compactionModel.SetDisableThinking(true)
	compactionModel.SetMaxTokens(2000)
	b.ctxMgr.SetCompactionModel(compactionModel)

	b.ag = agent.New(context.Background(), agent.Config{
		Context: b.ctxMgr,
		Model:   llmClient,
		Tools:   b.toolbox.Registry,
		Policy:  b.policy,
		Output: agent.Output{
			Text:   b.stream,
			Reason: b.reasoning,
			Phase:  b.phase,
		},
		Interaction: agent.Interaction{
			Confirm: b.confirm,
			Ask:     b.ask,
			Todos: func(items []protocol.TodoItem) {
				b.ctxMgr.SetTodos(items)
				b.todos(items)
			},
		},
	})
	b.configureAgent(b.ag)

	// Inject the permission engine into the shell tool so builtin sandbox
	// rules (e.g. pnpm dev → network) are applied.
	b.toolbox.SetSandboxProfiler(b.ag.SandboxProfiler())

	b.wireTaskTool(fm, compactionModel, b.ag)
}

func (b *Bot) initCommands() {
	deps := command.Deps{
		CtxMgr:            b.ctxMgr,
		Ag:                func() command.PlanModeController { return b.getAgent().Executor() },
		Skills:            b.ext,
		ToolRegistry:      b.toolbox.Registry,
		BaseSystemPrompt:  b.promptBuilder.BuildStatic,
		GetConfigFn:       b.model,
		ListModelsFn:      b.cfg.AllModelNames,
		SwitchModel:       b.SwitchModel,
		ResetConversation: b.resetConversation,
	}
	if b.cmd == nil {
		b.cmd = command.New(deps)
	} else {
		b.cmd.RegisterAll(deps)
	}

	b.ext.RegisterCommands(b.cmd.Parser(), b.confirmInstall)
	b.registerSessionCommands(b.cmd.Parser())
}
