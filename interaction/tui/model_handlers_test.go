package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type commandFakeBot struct {
	tickFakeBot
	commands []string
}

func (b *commandFakeBot) CommandCatalog() []string {
	return append([]string(nil), b.commands...)
}

func TestEnterRunsSelectedCommandFromSplash(t *testing.T) {
	bot := &commandFakeBot{commands: []string{"/help", "/status"}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}

	m.Input.SetValue("/")
	m.refreshSuggestions()
	if !m.Suggestions.Visible() {
		t.Fatal("command suggestions should be visible")
	}

	cmd := m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if got := bot.submittedInputs(); len(got) != 1 || got[0] != "/help" {
		t.Fatalf("submitted inputs = %#v, want selected /help command", got)
	}
	if cmd == nil {
		t.Fatal("selected command should start the processing tick")
	}
}
