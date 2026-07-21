// agent.go — 启动 agent 对话流程：startChat、startAgent。
package tui

import (
	"context"
	"fmt"
	"strings"

	commonview "nekocode/common/view"
	"nekocode/interaction/tui/components/block"
	"nekocode/interaction/tui/components/message"
	controlruntime "nekocode/runtime"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) startChat(value string) tea.Cmd {
	m.transitionTo(stateProcessing)
	m.Messages.SetSpinnerView(m.Spinner.View())
	status := PhaseWaiting
	if isSummarizeCommand(value) {
		status = phaseSummarizing
	}
	m.setPhase(status)
	m.Messages.SetProcessingStatus(status)
	if _, err := m.Runtime.Submit(context.Background(), controlruntime.Input{
		Kind:   controlruntime.InputMessage,
		Source: controlruntime.SourceRef{Kind: "tui"},
		Text:   value,
	}); err != nil {
		m.transitionTo(stateReady)
		m.Messages.AddMessage(message.ChatMessage{
			Role:    "error",
			Content: err.Error(),
		})
		return nil
	}
	return spinnerTick()
}

func isSummarizeCommand(value string) bool {
	return strings.TrimSpace(value) == "/summarize"
}

func (m *Model) onAgentStep(finalResponse *string) func(controlruntime.StepAction, string, string, string, bool) {
	return func(action controlruntime.StepAction, toolName, toolArgs, output string, isError bool) {
		switch {
		case action == controlruntime.StepActionThink:
		case action == controlruntime.StepActionChat:
			*finalResponse = output
			m.Messages.AddThinkBlock(output)
		case action == controlruntime.StepActionSubAgentStart:
			// toolName = subType, toolArgs = subID, output = colorIdx
			colorIdx := 0
			if n, err := fmt.Sscanf(output, "%d", &colorIdx); err != nil || n != 1 {
				colorIdx = -1
			}
			m.Messages.AddSubAgent(toolArgs, toolName, colorIdx)
		case action == controlruntime.StepActionSubAgentEnd:
			// toolArgs = subID
			m.Messages.RemoveSubAgent(toolArgs)
		case action == controlruntime.StepActionSubToolStart:
			if interactiveTool(toolName) {
				return
			}
			// toolName = actual tool name, toolArgs = args, output = subID:colorIdx
			subID, colorIdx := parseSubEvent(output)
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   commonview.ToolBrief(toolName, toolArgs),
				ToolAction: commonview.ToolAction(toolName, toolArgs),
				Content:    "",
				SubID:      subID,
				SubColor:   colorIdx,
			})
		case action == controlruntime.StepActionSubExecuteTool:
			if interactiveTool(toolName) {
				return
			}
			// toolName = actual tool name, output = text, toolArgs = subID:colorIdx
			subID, _ := parseSubEvent(toolArgs)
			m.Messages.AddSubToolOutput(subID, toolName, output, isError)
		case action == controlruntime.StepActionToolStart:
			if interactiveTool(toolName) {
				return
			}
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   commonview.ToolBrief(toolName, toolArgs),
				ToolAction: commonview.ToolAction(toolName, toolArgs),
				Content:    output,
			})
		case action == controlruntime.StepActionToolBlocked:
			if interactiveTool(toolName) {
				return
			}
			// Blocked by policy — create a completed error block showing the rejection reason.
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   commonview.ToolBrief(toolName, toolArgs),
				ToolAction: commonview.ToolAction(toolName, toolArgs),
				Content:    output,
				Done:       true,
				IsError:    true,
			})
		case action == controlruntime.StepActionToolPreview:
			if interactiveTool(toolName) {
				return
			}
			m.Messages.UpdateToolPreview(toolName, output)
		case toolName != "":
			if interactiveTool(toolName) {
				return
			}
			m.Messages.AddToolOutput(toolName, output, isError)
		}
	}
}

func interactiveTool(toolName string) bool {
	return toolName == "todo_write" || toolName == "question"
}

// parseSubEvent parses "subID:colorIdx" from event payload.
func parseSubEvent(payload string) (subID string, colorIdx int) {
	colorIdx = -1
	for i := len(payload) - 1; i >= 0; i-- {
		if payload[i] == ':' {
			if n, err := fmt.Sscanf(payload[i+1:], "%d", &colorIdx); err != nil || n != 1 {
				colorIdx = -1
			}
			subID = payload[:i]
			return
		}
	}
	subID = payload
	return
}

// loadSessionMessages populates the TUI message list from a restored session.
func (m *Model) loadSessionMessages() {
	for _, dm := range m.Runtime.SessionMessages() {
		var blocks []block.ContentBlock
		for _, b := range dm.Blocks {
			blocks = append(blocks, block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   b.ToolName,
				ToolArgs:   commonview.ToolBrief(b.ToolName, b.Args),
				ToolAction: commonview.ToolAction(b.ToolName, b.Args),
				Content:    b.Content,
				Done:       true,
				IsError:    b.IsError,
			})
		}
		m.Messages.AddMessage(message.ChatMessage{
			Role:            dm.Role,
			Content:         dm.Content,
			RenderedContent: dm.Content,
			Blocks:          blocks,
		})
	}
	m.Messages.GotoBottom()
}
