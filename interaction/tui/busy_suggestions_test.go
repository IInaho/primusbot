package tui

import (
	"context"
	"strings"
	"testing"

	controlruntime "nekocode/runtime"

	tea "charm.land/bubbletea/v2"
)

// busyLocalFakeBot offers command menus AND local command execution, so the
// busy-state suggestion flow can be exercised end to end.
type busyLocalFakeBot struct {
	commandFakeBot
	executed []string
}

func (b *busyLocalFakeBot) ExecuteLocalCommand(_ context.Context, input string) (string, controlruntime.LocalCommandResult) {
	input = strings.TrimSpace(input)
	b.executed = append(b.executed, input)
	return "ok: " + input, controlruntime.LocalCommandExecuted
}

func TestSuggestionsVisibleWhileBusy(t *testing.T) {
	bot := &busyLocalFakeBot{commandFakeBot: commandFakeBot{commands: []string{"/help", "/permission"}}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.transitionTo(stateProcessing)

	input, _ := m.Input.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	m.Input = input
	m.refreshSuggestions()
	if !m.Suggestions.Visible() {
		t.Fatal("typing / during a task must show the suggestion popup")
	}
}

func TestBusyEnterAcceptsSuggestionAndRunsLocal(t *testing.T) {
	bot := &busyLocalFakeBot{commandFakeBot: commandFakeBot{commands: []string{"/permission"}}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.transitionTo(stateProcessing)

	input, _ := m.Input.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	m.Input = input
	m.refreshSuggestions()
	if !m.Suggestions.Visible() {
		t.Fatal("suggestions should be visible")
	}
	// First Enter accepts the suggestion (completes "/permission " into the
	// input, mirroring the idle path); second Enter submits it.
	m.handleProcessingKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if got := strings.TrimSpace(m.Input.Value()); got != "/permission" {
		t.Fatalf("input after accept = %q", got)
	}
	m.handleProcessingKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if len(bot.executed) != 1 || bot.executed[0] != "/permission" {
		t.Fatalf("executed = %v, want [/permission]", bot.executed)
	}
	if bot.submittedInputs() != nil {
		t.Fatalf("local command must not steer/start a run: %v", bot.submittedInputs())
	}
}
