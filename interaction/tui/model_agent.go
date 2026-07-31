// agent.go — 启动 agent 对话流程：startChat、startAgent。
package tui

import (
	"context"
	"strings"

	"nekocode/interaction"
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
	if _, err := m.Runtime.StartRun(context.Background(), controlruntime.Input{
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

func interactiveTool(toolName string) bool {
	return toolName == "todo_write" || toolName == "question"
}

// loadSessionMessages populates the TUI message list from a restored session.
func (m *Model) loadSessionMessages() {
	m.Messages.SetProcessing(false)
	m.Messages.SetItems()
	for _, dm := range m.Runtime.SessionMessages() {
		var blocks []block.ContentBlock
		for _, b := range dm.Blocks {
			blocks = append(blocks, block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   b.ToolName,
				ToolArgs:   interaction.ToolBrief(b.ToolName, b.Args),
				ToolAction: interaction.ToolAction(b.ToolName, b.Args),
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
