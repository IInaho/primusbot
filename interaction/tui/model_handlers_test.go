package tui

import (
	"context"
	"strings"
	"testing"

	controlruntime "nekocode/runtime"

	tea "charm.land/bubbletea/v2"
)

type commandFakeBot struct {
	tickFakeBot
	commands []string
	menus    map[string]controlruntime.CommandMenu
}

func (b *commandFakeBot) CommandMenu(_ context.Context, input string) (controlruntime.CommandMenu, bool) {
	if input == "/" {
		items := make([]controlruntime.CommandMenuItem, 0, len(b.commands))
		for _, value := range b.commands {
			items = append(items, controlruntime.CommandMenuItem{Value: value, Label: value})
		}
		return controlruntime.CommandMenu{Title: "Commands", Items: items}, len(items) > 0
	}
	menu, ok := b.menus[input]
	return menu, ok
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

func TestEnterOpensCommandMenuAndSubmitsLeafChoice(t *testing.T) {
	bot := &commandFakeBot{
		commands: []string{"/model"},
		menus: map[string]controlruntime.CommandMenu{
			"/model": {
				Title: "Choose model",
				Items: []controlruntime.CommandMenuItem{
					{Value: "/model fast", Label: "fast", Description: "openai / gpt-fast", Submit: true},
				},
			},
		},
	}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/model")

	if cmd := m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})); cmd != nil {
		t.Fatal("opening a command menu started a run")
	}
	if !m.Suggestions.IsMenu() || !strings.Contains(m.Suggestions.View(80), "Choose model") {
		t.Fatalf("model menu did not open:\n%s", m.Suggestions.View(80))
	}

	cmd := m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("selecting a leaf choice did not start a run")
	}
	if got := bot.submittedInputs(); len(got) != 1 || got[0] != "/model fast" {
		t.Fatalf("submitted inputs = %#v", got)
	}
}

func TestNestedCommandMenuEscReturnsToParent(t *testing.T) {
	bot := &commandFakeBot{menus: map[string]controlruntime.CommandMenu{
		"/plugin": {
			Title: "Plugin action",
			Items: []controlruntime.CommandMenuItem{{Value: "/plugin enable", Label: "Enable"}},
		},
		"/plugin enable": {
			Title: "Choose plugin",
			Items: []controlruntime.CommandMenuItem{{Value: "/plugin enable demo", Label: "demo", Submit: true}},
		},
	}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/plugin")
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if !strings.Contains(m.Suggestions.View(80), "Choose plugin") {
		t.Fatalf("nested menu did not open:\n%s", m.Suggestions.View(80))
	}
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.Input.Value() != "/plugin" || !strings.Contains(m.Suggestions.View(80), "Plugin action") {
		t.Fatalf("escape did not restore parent: input=%q\n%s", m.Input.Value(), m.Suggestions.View(80))
	}
}
