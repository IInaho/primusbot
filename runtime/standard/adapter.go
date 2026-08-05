package standard

import (
	"context"

	"nekocode/bot/core"
	"nekocode/bot/extension"
	"nekocode/protocol"
	controlruntime "nekocode/runtime"
	"nekocode/runtime/agentrunner"
	"nekocode/runtime/standard/internal/viewmodel"
)

// adapter is the only standard-application boundary between the bot domain
// and runtime's interaction/read-model protocol.
type adapter struct {
	bot *core.Bot
}

func adapt(standardBot *core.Bot) *adapter {
	return &adapter{bot: standardBot}
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

func (a *adapter) CommandMenu(ctx context.Context, input string) (controlruntime.CommandMenu, bool) {
	return a.bot.CommandMenu(ctx, input)
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
	return viewmodel.DisplayMessages(snapshot.Messages)
}

func (a *adapter) ResumeSession(id string) error {
	return a.bot.ResumeSession(id)
}

func (a *adapter) NewSession() (controlruntime.SessionMeta, error) {
	snapshot, err := a.bot.NewSession()
	if err != nil {
		return controlruntime.SessionMeta{}, err
	}
	return viewmodel.SessionSnapshot(snapshot), nil
}

func (a *adapter) DeleteSession(id string) error {
	return a.bot.DeleteSession(id)
}

func (a *adapter) Close() error {
	return a.bot.Close()
}

func (a *adapter) services() controlruntime.Services {
	return controlruntime.Services{
		ExecuteCommand:         a.ExecuteCommand,
		CommandMenu:            a.CommandMenu,
		Steer:                  a.Steer,
		Metrics:                a.Metrics,
		CurrentModel:           a.CurrentModel,
		SwitchModel:            a.SwitchModel,
		ContextSnapshot:        a.ContextSnapshot,
		MemoryView:             a.MemoryView,
		SkillManagementView:    a.SkillManagementView,
		SelectSkill:            a.SelectSkill,
		ClearSelectedSkill:     a.ClearSelectedSkill,
		RefreshSkillManagement: a.RefreshSkillManagement,
		SetPluginEnabled:       a.SetPluginEnabled,
		ConfigView:             a.ConfigView,
		ApplyConfig:            a.ApplyConfig,
		CurrentSessionID:       a.CurrentSessionID,
		ListSessions:           a.ListSessions,
		SessionMessages:        a.SessionMessages,
		ResumeSession:          a.ResumeSession,
		NewSession:             a.NewSession,
		DeleteSession:          a.DeleteSession,
		Close:                  a.Close,
	}
}

var _ controlruntime.Runner = (*adapter)(nil)
