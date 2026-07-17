// agent.go — 启动 agent 对话流程：startChat、startAgent。
package tui

import (
	"context"
	"fmt"
	"strings"

	"nekocode/interaction/tui/components/block"
	"nekocode/interaction/tui/components/message"
	controlruntime "nekocode/runtime"
	"nekocode/util/runtime"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) startChat(value string) tea.Cmd {
	// /summarize involves a synchronous LLM call that can take 10+ seconds.
	// Show the command immediately and run the summarization in background
	// so the TUI stays responsive with a spinner.
	if isSummarizeCommand(value) {
		return m.startSummarize(value)
	}

	m.transitionTo(stateProcessing)
	m.Messages.SetSpinnerView(m.Spinner.View())
	m.Messages.SetProcessingStatus(PhaseWaiting)
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

func (m *Model) startSummarize(value string) tea.Cmd {
	m.transitionTo(stateProcessing)
	m.setPhase(phaseSummarizing)
	m.Messages.SetSpinnerView(m.Spinner.View())
	m.Messages.SetProcessingStatus(phaseSummarizing)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				runtime.WritePanicLog(r)
				m.summarizeCh <- summarizeDoneMsg{content: fmt.Sprintf("Summarize failed: internal panic: %v", r)}
			}
		}()
		resp, _ := m.Runtime.ExecuteCommand(value)
		m.summarizeCh <- summarizeDoneMsg{content: resp}
	}()

	return tea.Batch(
		spinnerTick(),
		listenSummarize(m.summarizeCh),
	)
}

func (m *Model) onAgentStep(finalResponse *string) func(string, string, string, string) {
	return func(action, toolName, toolArgs, output string) {
		switch {
		case action == "think":
		case action == "chat":
			*finalResponse = output
			m.Messages.AddThinkBlock(output)
		case action == "sub_agent_start":
			// toolName = subType, toolArgs = subID, output = colorIdx
			colorIdx := 0
			if n, err := fmt.Sscanf(output, "%d", &colorIdx); err != nil || n != 1 {
				colorIdx = -1
			}
			m.Messages.AddSubAgent(toolArgs, toolName, colorIdx)
		case action == "sub_agent_end":
			// toolArgs = subID
			m.Messages.RemoveSubAgent(toolArgs)
		case action == "sub_tool_start":
			if interactiveTool(toolName) {
				return
			}
			// toolName = actual tool name, toolArgs = args, output = subID:colorIdx
			subID, colorIdx := parseSubEvent(output)
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   formatBriefArgs(toolName, toolArgs),
				ToolAction: toolAction(toolName, toolArgs),
				Content:    "",
				SubID:      subID,
				SubColor:   colorIdx,
			})
		case action == "sub_execute_tool":
			if interactiveTool(toolName) {
				return
			}
			// toolName = actual tool name, output = text, toolArgs = subID:colorIdx
			subID, _ := parseSubEvent(toolArgs)
			m.Messages.AddSubToolOutput(subID, toolName, output)
		case action == "tool_start":
			if interactiveTool(toolName) {
				return
			}
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   formatBriefArgs(toolName, toolArgs),
				ToolAction: toolAction(toolName, toolArgs),
				Content:    output,
			})
		case action == "tool_blocked":
			if interactiveTool(toolName) {
				return
			}
			// Blocked by policy — create a completed error block showing the rejection reason.
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   formatBriefArgs(toolName, toolArgs),
				ToolAction: toolAction(toolName, toolArgs),
				Content:    output,
				Done:       true,
				IsError:    true,
			})
		case action == "tool_preview":
			if interactiveTool(toolName) {
				return
			}
			m.Messages.UpdateToolPreview(toolName, output)
		case toolName != "":
			if interactiveTool(toolName) {
				return
			}
			m.Messages.AddToolOutput(toolName, output)
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
				ToolArgs:   formatBriefArgs(b.ToolName, b.Args),
				ToolAction: toolAction(b.ToolName, b.Args),
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
