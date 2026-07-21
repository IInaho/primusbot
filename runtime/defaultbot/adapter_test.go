package defaultbot

import (
	"testing"
	"time"

	commonview "nekocode/common/view"
	controlruntime "nekocode/runtime"
)

type mockBot struct {
	configuredCallbacks commonview.ControlCallbacks
	runInput            string
	runCallbacks        commonview.RunCallbacks
	runResult           string
	runErr              error
}

func (m *mockBot) Run(input string, callbacks commonview.RunCallbacks) (string, error) {
	m.runInput = input
	m.runCallbacks = callbacks
	return m.runResult, m.runErr
}
func (m *mockBot) ConfigureRuntime(callbacks commonview.ControlCallbacks) {
	m.configuredCallbacks = callbacks
}
func (m *mockBot) ExecuteCommand(input string) (string, commonview.CmdResult) {
	return "", commonview.CmdNone
}
func (m *mockBot) SkillHint() (string, bool)                    { return "", false }
func (m *mockBot) CommandNames() []string                       { return nil }
func (m *mockBot) Steer(string)                                 {}
func (m *mockBot) Abort()                                       {}
func (m *mockBot) Close()                                       {}
func (m *mockBot) Stats() commonview.BotStats                   { return commonview.BotStats{PromptTokens: 1} }
func (m *mockBot) ProviderModel() (provider, model string)      { return "", "" }
func (m *mockBot) SessionMessages() []commonview.DisplayMessage { return nil }
func (m *mockBot) SwitchModel(name string) (model, provider string, err error) {
	return "", "", nil
}
func (m *mockBot) ContextStatus() string                       { return "" }
func (m *mockBot) ContextReport() string                       { return "" }
func (m *mockBot) ContextSnapshot() commonview.ContextSnapshot { return commonview.ContextSnapshot{} }
func (m *mockBot) MemoryView(scope commonview.MemoryScope) commonview.MemoryView {
	return commonview.MemoryView{}
}
func (m *mockBot) CWD() string                   { return "" }
func (m *mockBot) ClearContext()                 {}
func (m *mockBot) SelectSkill(name string) error { return nil }
func (m *mockBot) ClearSelectedSkill()           {}
func (m *mockBot) SkillManagementView() commonview.SkillManagementView {
	return commonview.SkillManagementView{}
}
func (m *mockBot) RefreshSkillManagement() commonview.SkillManagementView {
	return commonview.SkillManagementView{}
}
func (m *mockBot) SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error) {
	return commonview.SkillManagementView{}, nil
}
func (m *mockBot) ConfigView() commonview.ConfigView { return commonview.ConfigView{} }
func (m *mockBot) ApplyConfig(cfg commonview.ConfigView) (commonview.ConfigView, error) {
	return cfg, nil
}
func (m *mockBot) CurrentSessionID() string               { return "" }
func (m *mockBot) SetSession(id string) error             { return nil }
func (m *mockBot) ResumeSession(id string) error          { return nil }
func (m *mockBot) ListSessions() []commonview.SessionMeta { return nil }
func (m *mockBot) NewSession() (commonview.SessionMeta, error) {
	return commonview.SessionMeta{}, nil
}
func (m *mockBot) DeleteSession(id string) error { return nil }

func TestAdapterForwardsRunCallbacks(t *testing.T) {
	bot := &mockBot{runResult: "result"}
	a := &adapter{bot: bot}

	var deltas []string
	result, err := a.Run("hello", controlruntime.RunCallbacks{
		Text: func(delta string) { deltas = append(deltas, delta) },
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result != "result" {
		t.Fatalf("Run result = %q, want %q", result, "result")
	}
	if bot.runInput != "hello" {
		t.Fatalf("bot input = %q, want %q", bot.runInput, "hello")
	}

	// Exercise the callback bridge.
	bot.runCallbacks.Text("delta")
	if len(deltas) != 1 || deltas[0] != "delta" {
		t.Fatalf("text deltas = %v, want [delta]", deltas)
	}
}

func TestAdapterConfiguresConfirmCh(t *testing.T) {
	bot := &mockBot{}
	a := &adapter{bot: bot}

	coreCh := make(chan controlruntime.ConfirmRequest, 1)
	a.ConfigureRuntime(controlruntime.ControlCallbacks{
		ConfirmCh: coreCh,
	})

	if bot.configuredCallbacks.ConfirmCh == nil {
		t.Fatal("ConfirmCh was not forwarded")
	}

	// Send an unblock signal from the bot side and verify it reaches the runtime.
	bot.configuredCallbacks.ConfirmCh <- commonview.ConfirmRequest{}
	var req controlruntime.ConfirmRequest
	select {
	case req = <-coreCh:
	case <-time.After(time.Second):
		t.Fatal("unblock signal was not forwarded to runtime ConfirmCh")
	}
	if req.Response != nil {
		t.Fatalf("expected nil Response for unblock signal, got %v", req.Response)
	}
}

func TestAdapterStatsConversion(t *testing.T) {
	bot := &mockBot{}
	a := &adapter{bot: bot}

	stats := a.Stats()
	if stats.PromptTokens != 1 {
		t.Fatalf("PromptTokens = %d, want 1", stats.PromptTokens)
	}
}
