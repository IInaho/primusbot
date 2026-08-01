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

func TestEnterCompletesSelectedCommandBeforeSubmitting(t *testing.T) {
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

	if got := bot.submittedInputs(); len(got) != 0 {
		t.Fatalf("submitted inputs = %#v, want no submission while accepting a suggestion", got)
	}
	if cmd != nil {
		t.Fatal("accepting a suggestion should not start the processing tick")
	}
	if got := m.Input.Value(); got != "/help" {
		t.Fatalf("input value = %q, want selected /help command", got)
	}
	if m.Suggestions.Visible() {
		t.Fatal("suggestions should close after accepting a command")
	}

	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: 'v', Text: "verbose"}))
	if got := m.Input.Value(); got != "/help verbose" {
		t.Fatalf("input value after typing an argument = %q, want %q", got, "/help verbose")
	}

	cmd = m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if got := bot.submittedInputs(); len(got) != 1 || got[0] != "/help verbose" {
		t.Fatalf("submitted inputs = %#v, want completed command with arguments", got)
	}
	if cmd == nil {
		t.Fatal("second enter should start the processing tick")
	}
}
