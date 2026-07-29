package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nekocode/bot/tools/runtime/core"
	"nekocode/common/debug"
)

const fallbackNoAction = "Sorry, I couldn't determine what to do"

type actionType int

const (
	actionChat actionType = iota
	actionExecuteTool
)

func (a actionType) String() string {
	switch a {
	case actionChat:
		return "chat"
	case actionExecuteTool:
		return "execute_tool"
	default:
		return "unknown"
	}
}

type reasoningResult struct {
	Thought         string
	Action          actionType
	ActionInput     string
	ToolCalls       []core.ToolCallItem
	TextContent     string
	Interrupted     bool
	GarbledToolCall bool
	IsError         bool
}

func commandResult() *reasoningResult {
	return &reasoningResult{Thought: "User entered a command", Action: actionChat}
}

func fromLLM(toolCalls []core.ToolCallItem, textContent string, err error) *reasoningResult {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &reasoningResult{Thought: "User interrupted", Action: actionChat, Interrupted: true}
		}
		if textContent != "" && !isGarbledToolCall(textContent) {
			return &reasoningResult{Thought: "Truncated reply", Action: actionChat, ActionInput: textContent}
		}
		return &reasoningResult{Thought: "LLM call failed", Action: actionChat, ActionInput: fmt.Sprintf("LLM call failed: %v", err), IsError: true}
	}

	if len(toolCalls) == 0 {
		if isGarbledToolCall(textContent) {
			debug.Log("GarbledToolCall: XML leaked (len=%d)", len(textContent))
			return &reasoningResult{Thought: "Format correction", Action: actionChat, GarbledToolCall: true}
		}
		if textContent == "" {
			textContent = fallbackNoAction
		}
		return &reasoningResult{Thought: "Direct reply", Action: actionChat, ActionInput: textContent}
	}

	if len(toolCalls) == 1 {
		tc := toolCalls[0]
		return &reasoningResult{
			Thought:     "Call tool: " + tc.Name,
			Action:      actionExecuteTool,
			ActionInput: tc.Name + ":" + core.FormatArgs(tc.Args),
			ToolCalls:   toolCalls,
			TextContent: textContent,
		}
	}

	var names []string
	for _, tc := range toolCalls {
		names = append(names, tc.Name)
	}
	return &reasoningResult{
		Thought:     "Parallel tool calls: " + strings.Join(names, ", "),
		Action:      actionExecuteTool,
		ToolCalls:   toolCalls,
		TextContent: textContent,
	}
}

func isGarbledToolCall(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if strings.Contains(t, "<invoke") || strings.Contains(t, "</invoke") ||
		strings.Contains(t, "<parameter") || strings.Contains(t, "</parameter") ||
		strings.Contains(t, "<tool_call") || strings.Contains(t, "</tool_call") {
		return true
	}
	return strings.Contains(t, `"tool_calls"`) || strings.Contains(t, `"tool_use"`)
}
