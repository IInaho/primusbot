// model.go — Model 结构体 + 初始化 + 状态切换。
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nekocode/interaction/tui/components"
	"nekocode/interaction/tui/styles"
	controlruntime "nekocode/runtime"
	"nekocode/util/version"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type RuntimeClient interface {
	controlruntime.Interaction
	SteerRun(context.Context, controlruntime.RunID, controlruntime.Input) error
	ExecuteLocalCommand(context.Context, string) (string, controlruntime.LocalCommandResult)
	CurrentModel() controlruntime.ModelSelection
	PermissionMode() string
	SessionMessages() []controlruntime.DisplayMessage
	Close() error
}

type Model struct {
	Runtime  RuntimeClient
	Header   *components.Header
	Messages *components.Messages
	Input    *components.Input
	Splash   *components.Splash
	Spinner  spinner.Model
	Width    int
	Height   int
	Ready    bool

	state           chatState
	preConfirmState chatState
	processingStart time.Time
	processingPhase string
	Suggestions     *components.Suggestions
	ConfirmBar      *components.ConfirmBar
	QuestionBar     *components.QuestionBar
	Scrollbar       *components.Scrollbar
	runtimeEvents   <-chan controlruntime.Event
	metrics         controlruntime.MetricsSnapshot
	commandMenuBack []string
}

var displayVersion = version.Version

func NewModel(rt RuntimeClient) (*Model, error) {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sty := styles.DefaultStyles()

	var prov, mod string
	selection := rt.CurrentModel()
	prov, mod = selection.Provider, selection.Model
	events, err := rt.Events(context.Background(), controlruntime.EventFilter{})
	if err != nil {
		return nil, fmt.Errorf("subscribe runtime events: %w", err)
	}
	m := &Model{
		Runtime:       rt,
		Header:        components.NewHeader(80, prov, mod, displayVersion),
		Messages:      components.NewMessages(80, 14, &sty),
		Input:         components.NewInput(80),
		Splash:        components.NewSplash(80, 24, displayVersion),
		Spinner:       sp,
		Suggestions:   components.NewSuggestions(&sty),
		ConfirmBar:    components.NewConfirmBar(&sty),
		QuestionBar:   components.NewQuestionBar(&sty),
		Scrollbar:     components.NewScrollbar(&sty),
		Width:         80,
		Height:        24,
		state:         stateReady,
		runtimeEvents: events,
	}
	m.Input.SetHistory(loadInputHistory())
	m.Input.SetPermissionMode(rt.PermissionMode())

	return m, nil
}

func (m *Model) Init() tea.Cmd { return tea.Batch(m.Input.Init(), listenRuntimeEvent(m.runtimeEvents)) }

func (m *Model) resizeMessages() {
	follow := m.Messages.Follow
	extra := 0
	if m.state == stateConfirming {
		extra += m.ConfirmBar.Height(m.Width, m.Height)
	}
	if m.state == stateQuestioning {
		extra += m.QuestionBar.Height(m.Width, m.Height)
	}
	if m.Suggestions.Visible() {
		extra += m.Suggestions.Height()
	}
	msgHeight := m.Height - m.Header.Height() - m.Input.Height() - contentMarginV - extra
	if msgHeight < 0 {
		msgHeight = 0
	}
	// Horizontal scrollbar occupies 1 row above the header when content
	// overflows the viewport; reserve it so the message area shrinks.
	if m.Messages.TotalContentHeight() > msgHeight && msgHeight > 0 {
		msgHeight--
	}
	width := m.Width
	m.Messages.SetSize(width, msgHeight)
	if follow {
		m.Messages.GotoBottom()
	}
}

func (m *Model) transitionTo(state chatState) {
	m.state = state
	switch state {
	case stateReady:
		m.setPhase(PhaseReady)
		m.Messages.SetProcessing(false)
		m.Input.SetSending(false)
		m.ConfirmBar.Clear()
		m.QuestionBar.Clear()
	case stateProcessing:
		m.processingStart = time.Now()
		m.setPhase(PhaseWaiting)
		m.Messages.SetProcessingStatus(PhaseWaiting)

		m.Messages.SetProcessing(true)
		m.Input.SetSending(true)
	case stateConfirming:
	}
	m.resizeMessages()
}

func listenRuntimeEvent(ch <-chan controlruntime.Event) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return runtimeEventMsg{event: ev}
	}
}

// Processing phases displayed in the status line during agent execution.
const (
	phaseSteer       = "Processing new input..."
	phaseSummarizing = "Summarizing context..."
	PhaseReady       = "Ready"
	PhaseWaiting     = "Waiting"
)

func (m *Model) setPhase(p string) {
	if m.processingPhase == phaseSteer && p == PhaseWaiting {
		return
	}
	m.processingPhase = p
}

func todoItemsText(items []controlruntime.TodoItem) string {
	if len(items) == 0 {
		return ""
	}
	done := countCompleted(items)
	if done == len(items) {
		return fmt.Sprintf("✓ All %d tasks complete", done)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tasks %d/%d", done, len(items))
	for _, it := range items {
		fmt.Fprintf(&b, "\n%s %s", todoStatusIcon(it.Status), it.Content)
	}
	return b.String()
}

func countCompleted(items []controlruntime.TodoItem) int {
	n := 0
	for _, it := range items {
		if it.Status == "completed" {
			n++
		}
	}
	return n
}

func todoStatusIcon(status string) string {
	switch status {
	case "in_progress":
		return "▸"
	case "completed":
		return "✓"
	default:
		return "·"
	}
}
