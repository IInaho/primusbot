// handlers.go — 按键处理 + 完成处理 + spinner tick + 调试日志。
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nekocode/interaction"
	"nekocode/interaction/tui/components/block"
	"nekocode/interaction/tui/components/message"
	"nekocode/interaction/tui/components/processing"
	controlruntime "nekocode/runtime"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

const (
	contentMarginV = 2
)

// --- done ---

func (m *Model) handleDone(msg doneMsg) tea.Cmd {
	finalBlocks := block.FilterFinalBlocks(m.Messages.ProcessingBlocks())

	// Use msg.content (the final chat output) as primary rendered content.
	// AccumulatedText() may include intermediate turn text when Stop hooks
	// trigger additional agent loops, so fall back to it only when final output
	// is empty.
	accumulated := strings.TrimSpace(msg.content)
	if accumulated == "" {
		accumulated = strings.TrimSpace(m.Messages.AccumulatedText())
	}
	m.transitionTo(stateReady)

	if msg.err != nil {
		// Preserve tool blocks even on error — show what was attempted.
		if len(finalBlocks) > 0 {
			m.Messages.AddMessage(message.ChatMessage{
				Role:   "assistant",
				Blocks: finalBlocks,
			})
		}
		m.Messages.AddMessage(message.ChatMessage{
			Role:    "error",
			Content: fmt.Sprintf("Error: %v", msg.err),
		})
	} else {
		if accumulated == "" && len(finalBlocks) == 0 {
			st := m.metrics
			m.Header.SetTokens(st.PromptTokens + st.CompletionTokens)
			return nil
		}
		footer := ""
		if msg.duration != "" || msg.tokens != "" {
			footer = "Duration: " + msg.duration
			if msg.tokens != "" {
				footer += "  " + msg.tokens
			}
		}
		m.Messages.AddMessage(message.ChatMessage{
			Role:            "assistant",
			Content:         msg.content,
			RenderedContent: accumulated,
			Footer:          footer,
			Blocks:          finalBlocks,
		})
	}

	st := m.metrics
	m.Header.SetTokens(st.PromptTokens + st.CompletionTokens)
	if m.Messages.Follow {
		m.Messages.GotoBottom()
	}
	return nil
}

// --- keys: confirm ---

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.ConfirmBar.Submit()
	case "up":
		m.ConfirmBar.Move(-1)
		return m, nil
	case "down":
		m.ConfirmBar.Move(1)
		return m, nil
	case "esc", "n", "N", "ctrl+c":
		m.ConfirmBar.Respond(false, false)
	default:
		return m, nil
	}
	m.state = m.preConfirmState
	m.resizeMessages()
	if m.state == stateProcessing {
		return m, spinnerTick()
	}
	return m, nil
}

func (m *Model) handleQuestionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		m.QuestionBar.Move(-1)
		return m, nil
	case "down", "tab":
		m.QuestionBar.Move(1)
		return m, nil
	case "backspace":
		m.QuestionBar.Backspace()
		return m, nil
	case "space":
		if m.QuestionBar.CustomActive() {
			m.QuestionBar.Type(" ")
			return m, nil
		}
		m.QuestionBar.Toggle()
		return m, nil
	case "enter":
		m.QuestionBar.Submit()
	case "esc", "ctrl+c":
		m.QuestionBar.Reject()
	default:
		if len([]rune(msg.String())) == 1 {
			m.QuestionBar.Type(msg.String())
		}
		return m, nil
	}
	m.state = m.preConfirmState
	m.resizeMessages()
	if m.state == stateProcessing {
		return m, spinnerTick()
	}
	return m, nil
}

// --- keys: question ---

// --- keys: dispatch ---

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit

	case "up":
		if m.Suggestions.Visible() {
			m.Suggestions.Cycle(-1)
		} else if m.state == stateProcessing {
			m.Messages.Update(msg)
		} else if m.Input.CanCursorUp() {
			input, cmd := m.Input.Update(msg)
			m.Input = input
			m.resizeMessages()
			return cmd
		} else {
			m.Input.HistoryUp()
			m.resizeMessages()
		}
		return nil
	case "down":
		if m.Suggestions.Visible() {
			m.Suggestions.Cycle(1)
		} else if m.state == stateProcessing {
			m.Messages.Update(msg)
		} else if m.Input.CanCursorDown() {
			input, cmd := m.Input.Update(msg)
			m.Input = input
			m.resizeMessages()
			return cmd
		} else {
			m.Input.HistoryDown()
			m.resizeMessages()
		}
		return nil

	case "pgup", "pgdown":
		m.Messages.Update(msg)
		m.Input.SetFollow(m.Messages.Follow)
		return nil
	}

	if m.state == stateProcessing {
		return m.handleProcessingKey(msg)
	}

	return m.handleIdleKey(msg)
}

func (m *Model) handleProcessingKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		value := m.Input.Value()
		if value != "" {
			m.Suggestions.Hide()
			m.resizeMessages()
			m.rememberInput(value)
			m.Input.Reset()
			m.Messages.GotoBottom()
			m.Input.SetFollow(true)
			m.processingStart = time.Now()
			m.processingPhase = phaseSteer
			m.Messages.SetProcessingStatus(phaseSteer)
			if err := m.Runtime.SteerRun(context.Background(), "", controlruntime.Input{
				Source: controlruntime.SourceRef{Kind: "tui"},
				Text:   value,
			}); err != nil {
				m.Messages.AddMessage(message.ChatMessage{Role: "error", Content: err.Error()})
			}
		}
	case "esc":
		if err := m.Runtime.CancelRun(context.Background(), ""); err != nil {
			m.Messages.AddMessage(message.ChatMessage{Role: "error", Content: err.Error()})
		} else {
			m.Messages.SetProcessingStatus("Aborted")
		}
	default:
		input, cmd := m.Input.Update(msg)
		m.Input = input
		m.resizeMessages()
		return cmd
	}
	return nil
}

func (m *Model) handleIdleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "end":
		m.Messages.GotoBottom()
		m.Input.SetFollow(true)
	case "tab":
		m.cycleSuggestion(1)
		return nil
	case "shift+tab":
		m.cycleSuggestion(-1)
		return nil
	case "esc":
		if m.Suggestions.Visible() {
			m.Suggestions.Hide()
			m.resizeMessages()
			return nil
		}
	case "enter":
		if m.Suggestions.Visible() {
			if selected := m.Suggestions.Accept(); selected != "" {
				m.Input.SetValue(selected + " ")
				m.Input.SetCursorEnd()
			}
			m.resizeMessages()
			return nil
		}
		value := m.Input.Value()
		if value == "" {
			m.Messages.GotoBottom()
			m.Input.SetFollow(true)
			return nil
		}
		m.Suggestions.Hide()
		m.resizeMessages()
		m.rememberInput(value)
		m.Input.Reset()
		return m.startChat(value)
	default:
		input, cmd := m.Input.Update(msg)
		m.Input = input
		m.refreshSuggestions()
		return cmd
	}
	return nil
}

// --- suggestions ---

func (m *Model) refreshSuggestions() {
	m.Suggestions.Refresh(m.Input.Value(), m.Runtime.CommandCatalog())
	m.resizeMessages()
}

func (m *Model) cycleSuggestion(delta int) {
	m.Suggestions.Cycle(delta)
}

// --- spinner ---

func (m *Model) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	m.Spinner, _ = m.Spinner.Update(msg)

	if m.state == stateConfirming {
		m.Messages.SetSpinnerView("")
		return nil
	}
	if m.state == stateQuestioning {
		m.Messages.SetSpinnerView("")
		return nil
	}

	if m.state == stateProcessing {
		elapsed := time.Since(m.processingStart)
		statusText := fmt.Sprintf("%s (%.1fs)", m.processingPhase, elapsed.Seconds())
		spinnerView := m.Spinner.View()
		m.Messages.UpdateProcessing(func(p *processing.ProcessingItem) {
			p.SetSpinnerView(spinnerView)
			p.SetStatusText(statusText)
		})
		if m.processingPhase != phaseSummarizing {
			st := m.metrics
			m.Messages.UpdateProcessing(func(p *processing.ProcessingItem) {
				p.SetTokens(st.TurnPrompt, st.TurnCompletion)
				p.SetCompactCount(st.CompactCount)
			})
		}
		return spinnerTick()
	}

	return nil
}

func spinnerTick() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return spinner.TickMsg{}
	}
}

func (m *Model) handleRuntimeEvent(ev controlruntime.Event) tea.Cmd {
	switch ev.Type {
	case controlruntime.EventInputAccepted:
		if p, ok := ev.Payload.(controlruntime.MessagePayload); ok {
			title := ""
			if p.Source.Kind != "" && p.Source.Kind != "tui" {
				title = "You · " + p.Source.Kind
				if p.Sender.Display != "" {
					title += " " + p.Sender.Display
				} else if p.Sender.Username != "" {
					title += " @" + p.Sender.Username
				}
			}
			m.Messages.AddMessage(message.ChatMessage{Role: "user", Title: title, Content: p.Content})
			m.Messages.ClearProcessing()
			m.Messages.SetBlocks(nil)
			m.Messages.GotoBottom()
			m.Input.SetFollow(true)
		}
	case controlruntime.EventSystemMessage:
		if p, ok := ev.Payload.(controlruntime.MessagePayload); ok && strings.TrimSpace(p.Content) != "" {
			m.Messages.AddMessage(message.ChatMessage{
				Role:            "system",
				Content:         p.Content,
				RenderedContent: p.Content,
			})
		}
		selection := m.Runtime.CurrentModel()
		m.Header.SetModel(selection.Provider, selection.Model)
	case controlruntime.EventRunStarted:
		if m.state != stateProcessing {
			m.transitionTo(stateProcessing)
		}
		m.metrics = controlruntime.MetricsSnapshot{}
		return spinnerTick()
	case controlruntime.EventAssistantDelta:
		if p, ok := ev.Payload.(controlruntime.DeltaPayload); ok {
			m.Messages.ProcessStreamText(p.Delta)
		}
	case controlruntime.EventReasoningDelta:
		if p, ok := ev.Payload.(controlruntime.DeltaPayload); ok {
			m.Messages.ProcessThinkingText(p.Delta)
		}
	case controlruntime.EventPhaseChanged:
		if p, ok := ev.Payload.(controlruntime.PhasePayload); ok {
			m.setPhase(p.Phase)
		}
	case controlruntime.EventMetricsUpdated:
		if metrics, ok := ev.Payload.(controlruntime.MetricsSnapshot); ok {
			m.metrics = metrics
		}
	case controlruntime.EventTodosUpdated:
		if items, ok := ev.Payload.([]controlruntime.TodoItem); ok {
			m.Messages.SetTodos(todoItemsText(items))
		}
	case controlruntime.EventToolStarted:
		m.applyRuntimeToolEvent(ev)
	case controlruntime.EventToolBlocked:
		m.applyRuntimeToolEvent(ev)
	case controlruntime.EventToolPreview:
		m.applyRuntimeToolEvent(ev)
	case controlruntime.EventToolCompleted:
		m.applyRuntimeToolEvent(ev)
	case controlruntime.EventSubAgentStarted:
		if p, ok := ev.Payload.(controlruntime.SubAgentPayload); ok {
			m.Messages.AddSubAgent(p.ID, p.Type, p.Color)
		}
	case controlruntime.EventSubAgentEnded:
		if p, ok := ev.Payload.(controlruntime.SubAgentPayload); ok {
			m.Messages.RemoveSubAgent(p.ID)
		}
	case controlruntime.EventApprovalRequested:
		if p, ok := ev.Payload.(controlruntime.ApprovalView); ok {
			req := p.ToConfirmRequest()
			m.ConfirmBar.SetRequest(&req, func(ok, remember bool) {
				_ = m.Runtime.DecideApproval(context.Background(), p.ID, controlruntime.ApprovalDecision{
					Allowed:  ok,
					Remember: ok && remember,
				})
			})
			m.preConfirmState = m.state
			m.state = stateConfirming
			m.resizeMessages()
		}
	case controlruntime.EventApprovalResolved:
		if m.state == stateConfirming {
			m.state = m.preConfirmState
			m.ConfirmBar.Clear()
			m.resizeMessages()
		}
	case controlruntime.EventQuestionRequested:
		if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
			req := p.ToQuestionRequest()
			m.QuestionBar.SetRequest(&req, func(reply controlruntime.QuestionReply) {
				_ = m.Runtime.AnswerQuestion(context.Background(), p.ID, reply)
			})
			m.preConfirmState = m.state
			m.state = stateQuestioning
			m.resizeMessages()
		}
	case controlruntime.EventQuestionResolved:
		if m.state == stateQuestioning {
			m.state = m.preConfirmState
			m.QuestionBar.Clear()
			m.resizeMessages()
		}
	case controlruntime.EventRunDone:
		payload, _ := ev.Payload.(controlruntime.RunResult)
		return m.handleDone(doneMsg{
			content:  payload.Output,
			duration: m.metrics.Duration,
			tokens:   tokensSummary(m.metrics),
		})
	case controlruntime.EventRunFailed:
		payload, _ := ev.Payload.(controlruntime.RunResult)
		err := errors.New("run failed")
		if payload.Error != "" {
			err = errors.New(payload.Error)
		}
		return m.handleDone(doneMsg{
			content:  payload.Output,
			duration: m.metrics.Duration,
			tokens:   tokensSummary(m.metrics),
			err:      err,
		})
	case controlruntime.EventRunCancelled:
		return m.handleDone(doneMsg{err: errors.New("cancelled")})
	case controlruntime.EventSessionChanged:
		m.loadSessionMessages()
	case controlruntime.EventConnectorStatus:
		if p, ok := ev.Payload.(controlruntime.ConnectorStatusPayload); ok && p.Message != "" {
			m.Messages.AddMessage(message.ChatMessage{
				Role:            "system",
				Content:         p.Message,
				RenderedContent: p.Message,
			})
		}
	}
	return nil
}

func (m *Model) applyRuntimeToolEvent(ev controlruntime.Event) {
	p, ok := ev.Payload.(controlruntime.ToolPayload)
	if !ok || interactiveTool(p.ToolName) {
		return
	}
	if p.SubAgentID != "" {
		switch ev.Type {
		case controlruntime.EventToolStarted:
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type: block.BlockTool, ToolName: p.ToolName,
				ToolArgs:   interaction.ToolBrief(p.ToolName, p.Args),
				ToolAction: interaction.ToolAction(p.ToolName, p.Args),
				SubID:      p.SubAgentID, SubColor: p.SubAgentColor,
			})
		case controlruntime.EventToolCompleted:
			m.Messages.AddSubToolOutput(p.SubAgentID, p.ToolName, p.Output, p.IsError)
		}
		return
	}
	switch ev.Type {
	case controlruntime.EventToolStarted:
		m.Messages.ProcessToolBlock(block.ContentBlock{
			Type: block.BlockTool, ToolName: p.ToolName,
			ToolArgs:   interaction.ToolBrief(p.ToolName, p.Args),
			ToolAction: interaction.ToolAction(p.ToolName, p.Args),
			Content:    p.Preview,
		})
	case controlruntime.EventToolBlocked:
		m.Messages.ProcessToolBlock(block.ContentBlock{
			Type: block.BlockTool, ToolName: p.ToolName,
			ToolArgs:   interaction.ToolBrief(p.ToolName, p.Args),
			ToolAction: interaction.ToolAction(p.ToolName, p.Args),
			Content:    p.Output, Done: true, IsError: true,
		})
	case controlruntime.EventToolPreview:
		m.Messages.UpdateToolPreview(p.ToolName, p.Preview)
	case controlruntime.EventToolCompleted:
		m.Messages.AddToolOutput(p.ToolName, p.Output, p.IsError)
	}
}
