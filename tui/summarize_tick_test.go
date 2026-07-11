package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"nekocode/bot/extension"
	"nekocode/bot/view"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type tickFakeBot struct{}

func (tickFakeBot) Run(string, view.RunCallbacks) (string, error) { return "", nil }
func (tickFakeBot) ExecuteCommand(string) (string, view.CmdResult) {
	time.Sleep(100 * time.Millisecond)
	return "summary done", view.CmdHandled
}
func (tickFakeBot) SkillHint() (string, bool) { return "", false }
func (tickFakeBot) Stats() view.BotStats      { return view.BotStats{} }
func (tickFakeBot) CommandNames() []string    { return nil }
func (tickFakeBot) Configure(view.ConfirmFunc, view.PhaseFunc, view.TodoFunc, func(string), chan view.ConfirmRequest, view.QuestionFunc) {
}
func (tickFakeBot) Steer(string)                    {}
func (tickFakeBot) Abort()                          {}
func (tickFakeBot) Close()                          {}
func (tickFakeBot) ProviderModel() (string, string) { return "test", "test" }
func (tickFakeBot) SwitchModel(string) (string, string, error) {
	return "", "", nil
}
func (tickFakeBot) ContextStatus() string                  { return "" }
func (tickFakeBot) ContextReport() string                  { return "" }
func (tickFakeBot) ContextSnapshot() view.ContextSnapshot  { return view.ContextSnapshot{} }
func (tickFakeBot) SelectSkill(string) error               { return nil }
func (tickFakeBot) ClearSelectedSkill()                    {}
func (tickFakeBot) SessionMessages() []view.DisplayMessage { return nil }
func (tickFakeBot) ConfigView() view.ConfigView            { return view.ConfigView{} }
func (tickFakeBot) ApplyConfig(view.ConfigView) (view.ConfigView, error) {
	return view.ConfigView{}, nil
}
func (tickFakeBot) SkillManagementView() extension.SkillManagementView {
	return extension.SkillManagementView{}
}
func (tickFakeBot) RefreshSkillManagement() extension.SkillManagementView {
	return extension.SkillManagementView{}
}
func (tickFakeBot) SetPluginEnabled(string, bool) (extension.SkillManagementView, error) {
	return extension.SkillManagementView{}, nil
}
func (tickFakeBot) CWD() string                           { return "" }
func (tickFakeBot) ClearContext()                         {}
func (tickFakeBot) CurrentSessionID() string              { return "" }
func (tickFakeBot) SetSession(string) error               { return nil }
func (tickFakeBot) ResumeSession(string) error            { return nil }
func (tickFakeBot) ListSessions() []view.SessionMeta      { return nil }
func (tickFakeBot) NewSession() (view.SessionMeta, error) { return view.SessionMeta{}, nil }
func (tickFakeBot) DeleteSession(string) error            { return nil }

type blockingStatsBot struct {
	tickFakeBot
	statsCalled chan struct{}
}

func (b blockingStatsBot) Stats() view.BotStats {
	close(b.statsCalled)
	select {}
}

func TestSummarizeProcessingTickUpdatesView(t *testing.T) {
	m := NewModel(tickFakeBot{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = model.(*Model)

	cmd := m.startSummarize("/summarize")
	if cmd == nil {
		t.Fatal("startSummarize should listen for completion")
	}

	before := fmt.Sprint(m.View())
	if !strings.Contains(before, "Summarizing context...") {
		t.Fatalf("summarize view missing status: %q", before)
	}

	model, _ = m.Update(spinner.TickMsg{})
	m = model.(*Model)
	afterOne := fmt.Sprint(m.View())
	model, _ = m.Update(spinner.TickMsg{})
	m = model.(*Model)
	afterTwo := fmt.Sprint(m.View())

	if before == afterOne {
		t.Fatalf("first processing tick did not change view")
	}
	if afterOne == afterTwo {
		t.Fatalf("second processing tick did not change view")
	}
}

func TestSummarizeProcessingTickDoesNotReadStats(t *testing.T) {
	bot := blockingStatsBot{statsCalled: make(chan struct{})}
	m := NewModel(bot)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = model.(*Model)

	m.transitionTo(stateProcessing)
	m.setPhase(phaseSummarizing)

	done := make(chan struct{})
	go func() {
		_ = m.handleSpinnerTick(spinner.TickMsg{})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("summarize processing tick blocked")
	}

	select {
	case <-bot.statsCalled:
		t.Fatal("summarize processing tick should not call Bot.Stats")
	default:
	}
}
