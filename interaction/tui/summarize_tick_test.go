package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	controlruntime "nekocode/runtime"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type tickFakeBot struct {
	mu        sync.Mutex
	submitted []string
}

func (b *tickFakeBot) Submit(_ context.Context, input controlruntime.Input) (controlruntime.RunID, error) {
	b.mu.Lock()
	b.submitted = append(b.submitted, input.Text)
	b.mu.Unlock()
	return "", nil
}
func (*tickFakeBot) Steer(context.Context, controlruntime.RunID, controlruntime.Input) error {
	return nil
}
func (*tickFakeBot) Abort(context.Context, controlruntime.RunID) error { return nil }
func (*tickFakeBot) Publish(controlruntime.Event)                      {}
func (*tickFakeBot) Approve(context.Context, string, controlruntime.ApprovalDecision) error {
	return nil
}
func (*tickFakeBot) Answer(context.Context, string, controlruntime.QuestionReply) error { return nil }
func (*tickFakeBot) Subscribe(context.Context, controlruntime.EventFilter) (<-chan controlruntime.Event, error) {
	return make(chan controlruntime.Event), nil
}
func (*tickFakeBot) Connect(context.Context, string, []string) (string, error) {
	return "", nil
}
func (*tickFakeBot) Disconnect(string) (string, error) {
	return "", nil
}
func (*tickFakeBot) CurrentRunView() (controlruntime.RunView, bool) {
	return controlruntime.RunView{}, false
}
func (*tickFakeBot) RunView(controlruntime.RunID) (controlruntime.RunView, bool) {
	return controlruntime.RunView{}, false
}
func (*tickFakeBot) ListRunViews(int) []controlruntime.RunView {
	return nil
}
func (*tickFakeBot) ArtifactView(controlruntime.RunID) (controlruntime.ArtifactView, bool) {
	return controlruntime.ArtifactView{}, false
}
func (*tickFakeBot) ConnectView() controlruntime.ConnectView {
	return controlruntime.ConnectView{}
}
func (*tickFakeBot) SkillHint() (string, bool) { return "", false }
func (*tickFakeBot) Stats() controlruntime.BotStats {
	return controlruntime.BotStats{}
}
func (*tickFakeBot) CommandNames() []string          { return nil }
func (*tickFakeBot) Close()                          {}
func (*tickFakeBot) ProviderModel() (string, string) { return "test", "test" }
func (*tickFakeBot) SwitchModel(string) (string, string, error) {
	return "", "", nil
}
func (*tickFakeBot) ContextStatus() string { return "" }
func (*tickFakeBot) ContextReport() string { return "" }
func (*tickFakeBot) ContextSnapshot() controlruntime.ContextSnapshot {
	return controlruntime.ContextSnapshot{}
}
func (*tickFakeBot) MemoryView(controlruntime.MemoryScope) controlruntime.MemoryView {
	return controlruntime.MemoryView{}
}
func (*tickFakeBot) SelectSkill(string) error { return nil }
func (*tickFakeBot) ClearSelectedSkill()      {}
func (*tickFakeBot) SessionMessages() []controlruntime.DisplayMessage {
	return nil
}
func (*tickFakeBot) ConfigView() controlruntime.ConfigView { return controlruntime.ConfigView{} }
func (*tickFakeBot) ApplyConfig(controlruntime.ConfigView) (controlruntime.ConfigView, error) {
	return controlruntime.ConfigView{}, nil
}
func (*tickFakeBot) SkillManagementView() controlruntime.SkillManagementView {
	return controlruntime.SkillManagementView{}
}
func (*tickFakeBot) RefreshSkillManagement() controlruntime.SkillManagementView {
	return controlruntime.SkillManagementView{}
}
func (*tickFakeBot) SetPluginEnabled(string, bool) (controlruntime.SkillManagementView, error) {
	return controlruntime.SkillManagementView{}, nil
}
func (*tickFakeBot) CWD() string                                { return "" }
func (*tickFakeBot) ClearContext()                              {}
func (*tickFakeBot) CurrentSessionID() string                   { return "" }
func (*tickFakeBot) SetSession(string) error                    { return nil }
func (*tickFakeBot) ResumeSession(string) error                 { return nil }
func (*tickFakeBot) ListSessions() []controlruntime.SessionMeta { return nil }
func (*tickFakeBot) NewSession() (controlruntime.SessionMeta, error) {
	return controlruntime.SessionMeta{}, nil
}
func (*tickFakeBot) DeleteSession(string) error { return nil }

func (b *tickFakeBot) submittedInputs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.submitted...)
}

type blockingStatsBot struct {
	tickFakeBot
	statsCalled chan struct{}
}

func (b *blockingStatsBot) Stats() controlruntime.BotStats {
	close(b.statsCalled)
	select {}
}

func TestSummarizeProcessingTickUpdatesView(t *testing.T) {
	bot := &tickFakeBot{}
	m := NewModel(bot)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = model.(*Model)

	cmd := m.startChat("/summarize")
	if cmd == nil {
		t.Fatal("startChat should tick while summarizing")
	}
	if got := bot.submittedInputs(); len(got) != 1 || got[0] != "/summarize" {
		t.Fatalf("submitted inputs = %#v, want /summarize", got)
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
	bot := &blockingStatsBot{statsCalled: make(chan struct{})}
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
