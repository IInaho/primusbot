package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nekocode/interaction/tui/components/message"
	controlruntime "nekocode/runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type commandFakeBot struct {
	tickFakeBot
	commands []string
	menus    map[string]controlruntime.CommandMenu
}

type statusFakeBot struct {
	tickFakeBot
	selection controlruntime.ModelSelection
}

type sessionDeleteFakeBot struct {
	commandFakeBot
	deleted   []string
	deleteErr error
}

func (b *sessionDeleteFakeBot) DeleteSession(id string) error {
	b.deleted = append(b.deleted, id)
	return b.deleteErr
}

func (b *statusFakeBot) CurrentModel() controlruntime.ModelSelection { return b.selection }

func TestSystemEventRefreshesInputModelFooter(t *testing.T) {
	bot := &statusFakeBot{selection: controlruntime.ModelSelection{Provider: "openai", Model: "old", ReasoningEffort: "low"}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	bot.selection.Model = "new"
	bot.selection.ReasoningEffort = "high"
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventSystemMessage})
	footer := ansi.Strip(m.Input.View())
	if !strings.Contains(footer, "openai/new") || !strings.Contains(footer, "Effort: high") || strings.Contains(footer, "openai/old") || strings.Contains(footer, "Effort: low") {
		t.Fatalf("model footer was not refreshed:\n%s", footer)
	}
}

func TestRunDoneAttachesCallUsageToAssistantTurn(t *testing.T) {
	bot := &statusFakeBot{}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventRunStarted})
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventMetricsUpdated, Payload: controlruntime.MetricsSnapshot{
		Duration: "2.1s", TurnTotal: 22_597, TurnInput: 21_865, TurnCached: 21_200,
		TurnNew: 665, TurnOutput: 732, TurnReasoning: 283, TurnCacheReported: true,
	}})
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventRunDone, Payload: controlruntime.RunResult{Output: "Done"}})

	items := m.Messages.Items()
	assistant, ok := items[len(items)-1].(*message.AssistantMessageItem)
	if !ok {
		t.Fatalf("last item = %T, want assistant message", items[len(items)-1])
	}
	clean := ansi.Strip(assistant.Render(180))
	for _, want := range []string{"↳ 2.1s", "总计 22.6k tok", "输入 21.9k", "缓存 21.2k", "未缓存 665", "输出 732", "推理 283", "本次命中 96.96%"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("assistant turn telemetry missing %q:\n%s", want, clean)
		}
	}
}

func TestRunFailureKeepsUsageAsStandaloneMetaLine(t *testing.T) {
	bot := &statusFakeBot{}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventRunStarted})
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventMetricsUpdated, Payload: controlruntime.MetricsSnapshot{
		TurnInput: 1000, TurnOutput: 12,
	}})
	m.handleRuntimeEvent(controlruntime.Event{Type: controlruntime.EventRunFailed, Payload: controlruntime.RunResult{Error: "provider failed"}})

	items := m.Messages.Items()
	telemetry, ok := items[len(items)-1].(*message.TelemetryMessageItem)
	if !ok {
		t.Fatalf("last item = %T, want telemetry message", items[len(items)-1])
	}
	clean := ansi.Strip(telemetry.Render(140))
	if !strings.Contains(clean, "输入 1.0k") || !strings.Contains(clean, "缓存 —") || !strings.Contains(clean, "输出 12") {
		t.Fatalf("failure telemetry missing usage:\n%s", clean)
	}
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

func TestEnterOnCurrentMenuItemHighlightsWithoutSubmitting(t *testing.T) {
	bot := &commandFakeBot{
		commands: []string{"/model"},
		menus: map[string]controlruntime.CommandMenu{
			"/model": {
				Title: "Choose model",
				Items: []controlruntime.CommandMenuItem{
					{Value: "/model flash", Label: "flash", Description: "zai / glm-flash", Submit: true, Current: true},
					{Value: "/model pro", Label: "pro", Description: "zai / glm-pro", Submit: true},
				},
			},
		},
	}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/model")
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if view := m.Suggestions.View(80); !strings.Contains(view, "✓") {
		t.Fatalf("current model row missing check mark:\n%s", view)
	}

	cmd := m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Fatal("entering on the current model started a run")
	}
	if got := bot.submittedInputs(); len(got) != 0 {
		t.Fatalf("submitted inputs = %#v, want no submission for the current model", got)
	}
	if !m.Suggestions.IsMenu() {
		t.Fatal("menu should stay open after refusing the current model")
	}
	if view := m.Suggestions.View(80); !strings.Contains(view, "already current") {
		t.Fatalf("missing 'already current' hint:\n%s", view)
	}

	m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if view := m.Suggestions.View(80); strings.Contains(view, "already current") {
		t.Fatalf("hint should clear after moving:\n%s", view)
	} else if !strings.Contains(view, "✓") {
		t.Fatalf("current model row should keep its check mark after moving:\n%s", view)
	}
	m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if got := bot.submittedInputs(); len(got) != 1 || got[0] != "/model pro" {
		t.Fatalf("submitted inputs = %#v, want /model pro", got)
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

func TestSessionMenuDeletesHighlightedSessionAfterConfirmation(t *testing.T) {
	bot := &sessionDeleteFakeBot{commandFakeBot: commandFakeBot{menus: map[string]controlruntime.CommandMenu{
		"/sessions": {
			Title: "Resume session",
			Items: []controlruntime.CommandMenuItem{
				{Key: "session_1", Value: "/sessions session_1", Label: "session_1", Submit: true},
				{Key: "session_2", Value: "/sessions session_2", Label: "session_2", Submit: true},
			},
		},
	}}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/sessions")
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if !strings.Contains(m.Suggestions.View(80), "d delete") {
		t.Fatalf("session menu does not advertise delete action:\n%s", m.Suggestions.View(80))
	}
	m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))

	m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	if m.state != stateConfirming || !strings.Contains(m.ConfirmBar.View(80, 24), "session_2") {
		t.Fatalf("delete confirmation not opened for highlighted session:\n%s", m.ConfirmBar.View(80, 24))
	}
	if len(bot.deleted) != 0 {
		t.Fatalf("deleted before confirmation: %#v", bot.deleted)
	}

	m.handleConfirmKey(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if len(bot.deleted) != 1 || bot.deleted[0] != "session_2" {
		t.Fatalf("deleted = %#v, want session_2", bot.deleted)
	}
	if m.state != stateReady || !m.Suggestions.IsMenu() {
		t.Fatal("session menu should reopen after deletion")
	}
}

func TestSessionDeleteConfirmationCanBeCancelled(t *testing.T) {
	bot := &sessionDeleteFakeBot{commandFakeBot: commandFakeBot{menus: map[string]controlruntime.CommandMenu{
		"/sessions": {
			Items: []controlruntime.CommandMenuItem{{Key: "session_1", Value: "/sessions session_1", Label: "session_1", Submit: true}},
		},
	}}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/sessions")
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	m.handleConfirmKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))

	if len(bot.deleted) != 0 {
		t.Fatalf("cancelled deletion called runtime: %#v", bot.deleted)
	}
	if m.state != stateReady || !m.Suggestions.IsMenu() {
		t.Fatal("session menu should reopen after cancellation")
	}
}

func TestSessionDeleteFailureIsShownAndMenuReopens(t *testing.T) {
	bot := &sessionDeleteFakeBot{
		commandFakeBot: commandFakeBot{menus: map[string]controlruntime.CommandMenu{
			"/sessions": {
				Items: []controlruntime.CommandMenuItem{{Key: "session_1", Value: "/sessions session_1", Label: "session_1", Submit: true}},
			},
		}},
		deleteErr: errors.New("session is locked"),
	}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/sessions")
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	m.handleConfirmKey(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))

	items := m.Messages.Items()
	if len(items) != 1 {
		t.Fatalf("message count = %d, want deletion error", len(items))
	}
	errItem, ok := items[0].(*message.ErrorMessageItem)
	if !ok || !strings.Contains(ansi.Strip(errItem.Render(80)), "session is locked") {
		t.Fatalf("deletion error not rendered: %#v", items[0])
	}
	if !m.Suggestions.IsMenu() {
		t.Fatal("session menu should reopen after deletion failure")
	}
}

func TestDStillTypesOutsideSessionMenu(t *testing.T) {
	m, err := NewModel(&tickFakeBot{})
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.handleIdleKey(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	if got := m.Input.Value(); got != "d" {
		t.Fatalf("input = %q, want d", got)
	}
}

func TestCtrlCClearsPopulatedInputBeforeQuitting(t *testing.T) {
	bot := &commandFakeBot{commands: []string{"/help", "/status"}}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Input.SetValue("/h")
	m.refreshSuggestions()
	if !m.Suggestions.Visible() {
		t.Fatal("command suggestions should be visible before clear")
	}

	cmd := m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd != nil {
		t.Fatal("ctrl+c with input should clear instead of quitting")
	}
	if m.Input.HasContent() || m.Input.Value() != "" {
		t.Fatalf("input after ctrl+c = %q, want empty", m.Input.Value())
	}
	if m.Suggestions.Visible() {
		t.Fatal("ctrl+c should close command suggestions")
	}
}

func TestCtrlCWithEmptyInputQuits(t *testing.T) {
	m, err := NewModel(&tickFakeBot{})
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}

	cmd := m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("ctrl+c with empty input should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command returned %T, want tea.QuitMsg", cmd())
	}
}

func TestCtrlCClearsSteeringInputWithoutLeavingProcessingState(t *testing.T) {
	m, err := NewModel(&tickFakeBot{})
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.transitionTo(stateProcessing)
	m.Input.SetValue("additional direction")

	if cmd := m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})); cmd != nil {
		t.Fatal("ctrl+c with steering input should not quit")
	}
	if m.Input.HasContent() || m.state != stateProcessing {
		t.Fatalf("after clear: input=%q state=%v, want empty processing state", m.Input.Value(), m.state)
	}
}

func TestEnterSubmitsExpandedLargePaste(t *testing.T) {
	bot := &tickFakeBot{}
	m, err := NewModel(bot)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	content := strings.Repeat("pasted diagnostic line\n", 12) + "last line"
	model, _ := m.Update(tea.PasteMsg{Content: content})
	m = model.(*Model)

	if cmd := m.handleKeyPress(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})); cmd == nil {
		t.Fatal("enter after a large paste should start a run")
	}
	if got := bot.submittedInputs(); len(got) != 1 || got[0] != content {
		t.Fatalf("submitted input did not preserve pasted content: got %d entries", len(got))
	}
}
