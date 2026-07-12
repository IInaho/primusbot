// model.go — Model 结构体 + 初始化 + 状态切换。
package tui

import (
	"fmt"
	"strings"
	"time"

	"nekocode/bot"
	"nekocode/bot/view"
	"nekocode/tui/components"
	"nekocode/tui/styles"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

)

type Model struct {
	Bot      bot.UI
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
	activeSkill     string // skill activated this turn, shown in status bar
	Suggestions     *components.Suggestions
	ConfirmBar      *components.ConfirmBar
	QuestionBar     *components.QuestionBar
	Scrollbar       *components.Scrollbar
	confirmCh       chan view.ConfirmRequest
	questionCh      chan view.QuestionRequest
	notifyCh        chan string
	summarizeCh     chan summarizeDoneMsg
}

const version = "0.3.3"

func NewModel(b bot.UI) *Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sty := styles.DefaultStyles()

	prov, mod := b.ProviderModel()
	m := &Model{
		Bot:         b,
		Header:      components.NewHeader(80, prov, mod, version),
		Messages:    components.NewMessages(80, 14, &sty),
		Input:       components.NewInput(80),
		Splash:      components.NewSplash(80, 24, version),
		Spinner:     sp,
		Suggestions: components.NewSuggestions(&sty),
		ConfirmBar:  components.NewConfirmBar(&sty),
		QuestionBar: components.NewQuestionBar(&sty),
		Scrollbar:   components.NewScrollbar(&sty),
		Width:       80,
		Height:      24,
		state:       stateReady,
		confirmCh:   make(chan view.ConfirmRequest),
		questionCh:  make(chan view.QuestionRequest),
		notifyCh:    make(chan string, 8),
		summarizeCh: make(chan summarizeDoneMsg, 1),
	}
	m.Input.SetHistory(loadInputHistory())

	b.Configure(
		func(req view.ConfirmRequest) view.ConfirmReply {
			m.confirmCh <- req
			return <-req.Response
		},
		func(phase string) { m.setPhase(phase) },
		func(items []view.TodoItem) { m.Messages.SetTodos(todoItemsText(items)) },
		func(msg string) {
			select {
			case m.notifyCh <- msg:
			default:
			}
		},
		m.confirmCh,
		func(req view.QuestionRequest) view.QuestionReply {
			m.questionCh <- req
			return <-req.Response
		},
	)

	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.Input.Init(), listenNotify(m.notifyCh))
}

func (m *Model) resizeMessages() {
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

func listenConfirm(ch <-chan view.ConfirmRequest) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return confirmMsg{req: req}
	}
}

func listenQuestion(ch <-chan view.QuestionRequest) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return questionMsg{req: req}
	}
}

func listenNotify(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return notifyMsg{content: msg}
	}
}

func listenSummarize(ch <-chan summarizeDoneMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// Processing phases displayed in the status line during agent execution.
const (
	phaseSteer       = "Processing new input..."
	phaseSummarizing = "Summarizing context..."
	PhaseReady       = view.PhaseReady
	PhaseWaiting     = view.PhaseWaiting
)

func (m *Model) setPhase(p string) {
	if m.processingPhase == phaseSteer && p == PhaseWaiting {
		return
	}
	m.processingPhase = p
}

func todoItemsText(items []view.TodoItem) string {
	if len(items) == 0 {
		return ""
	}
	done := view.CountCompleted(items)
	if done == len(items) {
		return fmt.Sprintf("✓ All %d tasks complete", done)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Tasks %d/%d", done, len(items))
	for _, it := range items {
		fmt.Fprintf(&b, "\n%s %s", view.TodoStatusIcon(it.Status), it.Content)
	}
	return b.String()
}
