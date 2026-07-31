package standard

import (
	"context"
	"io"

	"nekocode/bot"
	"nekocode/bot/extension"
	"nekocode/protocol"
	controlruntime "nekocode/runtime"
	"nekocode/runtime/agentrunner"
	"nekocode/runtime/standard/internal/viewmodel"
)

// adapter is the only standard-application boundary between the bot domain
// and runtime's interaction/read-model protocol.
type adapter struct {
	bot *bot.Bot
}

func adapt(core *bot.Bot) *adapter {
	return &adapter{bot: core}
}

type runHost struct {
	controlruntime.RunHost
}

func (h runHost) Step(event protocol.StepEvent) {
	agentrunner.PublishStep(h.RunHost, event)
}

func (a *adapter) Run(ctx context.Context, input string, host controlruntime.RunHost) (string, error) {
	return a.bot.Run(ctx, input, runHost{RunHost: host})
}

func (a *adapter) ExecuteCommand(ctx context.Context, input string, host controlruntime.RunHost) (controlruntime.CommandResult, error) {
	return a.bot.ExecuteCommand(ctx, input, runHost{RunHost: host})
}

func (a *adapter) CommandNames() []string {
	return a.bot.CommandNames()
}

func (a *adapter) Steer(ctx context.Context, message string) error {
	return a.bot.Steer(ctx, message)
}

func (a *adapter) Metrics() controlruntime.MetricsSnapshot {
	return a.bot.Metrics()
}

func (a *adapter) CurrentModel() controlruntime.ModelSelection {
	config := a.bot.Configuration()
	return viewmodel.Model(config.ActiveModelConfig())
}

func (a *adapter) SwitchModel(name string) (controlruntime.ModelSelection, error) {
	if err := a.bot.SwitchModel(name); err != nil {
		return controlruntime.ModelSelection{}, err
	}
	return a.CurrentModel(), nil
}

func (a *adapter) ContextSnapshot() controlruntime.ContextSnapshot {
	return viewmodel.ContextSnapshot(a.bot.ContextReport())
}

func (a *adapter) MemoryView(scope controlruntime.MemoryScope) controlruntime.MemoryView {
	memory := a.bot.Memory()
	return viewmodel.Memory(scope, memory.Path, memory.Content)
}

func (a *adapter) SkillManagementView() controlruntime.SkillManagementView {
	return a.extensionView(a.bot.Extensions())
}

func (a *adapter) SelectSkill(name string) error {
	return a.bot.SelectSkill(name)
}

func (a *adapter) ClearSelectedSkill() {
	a.bot.ClearSelectedSkill()
}

func (a *adapter) RefreshSkillManagement() controlruntime.SkillManagementView {
	a.bot.RefreshExtensions()
	return a.SkillManagementView()
}

func (a *adapter) SetPluginEnabled(name string, enabled bool) (controlruntime.SkillManagementView, error) {
	if err := a.bot.SetPluginEnabled(name, enabled); err != nil {
		return controlruntime.SkillManagementView{}, err
	}
	return a.SkillManagementView(), nil
}

func (a *adapter) extensionView(snapshot extension.Snapshot) controlruntime.SkillManagementView {
	return viewmodel.Extension(snapshot, a.bot.Configuration().MCPServers)
}

func (a *adapter) ConfigView() controlruntime.ConfigView {
	return viewmodel.Config(a.bot.Configuration())
}

func (a *adapter) ApplyConfig(view controlruntime.ConfigView) (controlruntime.ConfigView, error) {
	if err := a.bot.ApplyConfiguration(viewmodel.ToConfig(view)); err != nil {
		return controlruntime.ConfigView{}, err
	}
	return a.ConfigView(), nil
}

func (a *adapter) CurrentSessionID() string {
	return a.bot.CurrentSessionID()
}

func (a *adapter) ListSessions() []controlruntime.SessionMeta {
	return viewmodel.SessionMetas(a.bot.ListSessions())
}

func (a *adapter) SessionMessages() []controlruntime.DisplayMessage {
	snapshot := a.bot.Conversation()
	return viewmodel.DisplayMessages(snapshot.Messages, snapshot.CompactBoundary)
}

func (a *adapter) ResumeSession(id string) error {
	return a.bot.ResumeSession(id)
}

func (a *adapter) NewSession() (controlruntime.SessionMeta, error) {
	return viewmodel.SessionSnapshot(a.bot.NewSession()), nil
}

func (a *adapter) DeleteSession(id string) error {
	return a.bot.DeleteSession(id)
}

func (a *adapter) Close() error {
	return a.bot.Close()
}

var (
	_ controlruntime.Runner               = (*adapter)(nil)
	_ controlruntime.Commander            = (*adapter)(nil)
	_ controlruntime.Steerer              = (*adapter)(nil)
	_ controlruntime.MetricsProvider      = (*adapter)(nil)
	_ controlruntime.ModelService         = (*adapter)(nil)
	_ controlruntime.ModelMutator         = (*adapter)(nil)
	_ controlruntime.ContextService       = (*adapter)(nil)
	_ controlruntime.ExtensionService     = (*adapter)(nil)
	_ controlruntime.ExtensionMutator     = (*adapter)(nil)
	_ controlruntime.ConfigurationService = (*adapter)(nil)
	_ controlruntime.ConfigurationMutator = (*adapter)(nil)
	_ controlruntime.SessionService       = (*adapter)(nil)
	_ controlruntime.SessionMutator       = (*adapter)(nil)
	_ io.Closer                           = (*adapter)(nil)
)
