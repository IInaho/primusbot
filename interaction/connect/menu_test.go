package connect

import (
	"context"
	"testing"

	"nekocode/protocol"
	controlruntime "nekocode/runtime"
)

type menuRuntime struct{}

func (menuRuntime) StartRun(context.Context, controlruntime.Input) (controlruntime.RunID, error) {
	return "", nil
}
func (menuRuntime) CancelRun(context.Context, controlruntime.RunID) error { return nil }
func (menuRuntime) DecideApproval(context.Context, string, controlruntime.ApprovalDecision) error {
	return nil
}
func (menuRuntime) AnswerQuestion(context.Context, string, protocol.QuestionReply) error { return nil }
func (menuRuntime) Events(context.Context, controlruntime.EventFilter) (<-chan controlruntime.Event, error) {
	return make(chan controlruntime.Event), nil
}
func (menuRuntime) ReportConnectorStatus(controlruntime.ConnectorStatusPayload) {}
func (menuRuntime) CommandMenu(_ context.Context, input string) (protocol.CommandMenu, bool) {
	switch input {
	case "/":
		return protocol.CommandMenu{Title: "Commands", Items: []protocol.CommandMenuItem{
			{Value: "/model", Label: "/model"},
			{Value: "/clear", Label: "/clear", Submit: true},
		}}, true
	case "/model":
		return protocol.CommandMenu{Title: "Models", Items: []protocol.CommandMenuItem{
			{Value: "/model fast", Label: "fast", Submit: true},
		}}, true
	default:
		return protocol.CommandMenu{}, false
	}
}

func TestCommandMenusResolveNestedAndNumericChoices(t *testing.T) {
	menus := NewCommandMenus()
	rt := menuRuntime{}
	root := menus.HandleText(context.Background(), rt, "chat-1", "/help")
	if root.Prompt == nil || len(root.Prompt.Choices) != 2 {
		t.Fatalf("root = %+v", root)
	}
	if invalid := menus.HandleText(context.Background(), rt, "chat-1", "99"); !invalid.Handled || invalid.Message == "" {
		t.Fatalf("invalid numeric selection = %+v", invalid)
	}

	nested := menus.Select(context.Background(), rt, "chat-1", root.Prompt.Choices[0].Token)
	if nested.Prompt == nil || nested.Prompt.Title != "Models" {
		t.Fatalf("nested = %+v", nested)
	}

	selected := menus.HandleText(context.Background(), rt, "chat-1", "1")
	if selected.Command != "/model fast" {
		t.Fatalf("numeric selection = %+v", selected)
	}
	replayed := menus.Select(context.Background(), rt, "chat-1", nested.Prompt.Choices[0].Token)
	if replayed.Message == "" || replayed.Command != "" {
		t.Fatalf("consumed command token replay = %+v", replayed)
	}
}

func TestCommandMenusRejectCrossConversationToken(t *testing.T) {
	menus := NewCommandMenus()
	root := menus.HandleText(context.Background(), menuRuntime{}, "chat-1", "/")
	got := menus.Select(context.Background(), menuRuntime{}, "chat-2", root.Prompt.Choices[1].Token)
	if got.Message == "" || !got.Handled {
		t.Fatalf("cross-conversation selection = %+v", got)
	}
}

func TestFormatMenuIncludesDescriptionsAndFallbackHint(t *testing.T) {
	text := FormatMenu(&MenuPrompt{Title: "Models", Choices: []MenuChoice{{Label: "fast", Description: "low latency"}}})
	if text != "Models\n\n1. fast — low latency\n\n回复序号进行选择。" {
		t.Fatalf("FormatMenu() = %q", text)
	}
}

func TestCommandMenusPaginateLargeRemoteMenus(t *testing.T) {
	menus := NewCommandMenus()
	items := make([]protocol.CommandMenuItem, 10)
	for i := range items {
		items[i] = protocol.CommandMenuItem{Value: "/pick " + string(rune('a'+i)), Label: string(rune('A' + i)), Submit: true}
	}
	first := menus.open("chat-1", protocol.CommandMenu{Title: "Choices", Items: items})
	if len(first.Choices) != menuPageSize+1 || first.Choices[len(first.Choices)-1].Label != "Next →" {
		t.Fatalf("first page = %+v", first)
	}
	second := menus.Select(context.Background(), menuRuntime{}, "chat-1", first.Choices[len(first.Choices)-1].Token)
	if second.Prompt == nil || second.Prompt.Title != "Choices · 2/2" {
		t.Fatalf("second page = %+v", second)
	}
	selected := menus.HandleText(context.Background(), menuRuntime{}, "chat-1", "1")
	if selected.Command != "/pick i" {
		t.Fatalf("second-page numeric selection = %+v", selected)
	}
}
