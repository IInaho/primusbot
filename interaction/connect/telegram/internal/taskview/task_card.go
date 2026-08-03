package taskview

import (
	"strings"
	"sync"

	"nekocode/interaction"
	"nekocode/interaction/connect"
	controlruntime "nekocode/runtime"
)

const maxRunCards = 20

// cardStatus is the lifecycle state of a tracked run card.
type cardStatus int

const (
	statusCreated cardStatus = iota
	statusAccepted
	statusRunning
	statusWaitingApproval
	statusWaitingQuestion
	statusDone
	statusFailed
	statusAborted
)

type Tracker struct {
	mu    sync.Mutex
	runs  map[controlruntime.RunID]*taskCard
	order []controlruntime.RunID
	last  controlruntime.RunID
}

type taskCard struct {
	Title     string
	Status    cardStatus
	LastPhase string
	Tools     []toolCard
	Diffs     []string
	Result    string
	Error     string
}

type toolCard struct {
	Name   string
	Brief  string
	Status string
}

func NewTracker() *Tracker {
	return &Tracker{
		runs: make(map[controlruntime.RunID]*taskCard),
	}
}

// Track records ev into the run cards — pure bookkeeping for /last /diff
// /status. It never produces push text: progress events are dashboard
// state, and outbound rendering lives in the connector's sink.
func (t *Tracker) Track(ev controlruntime.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch ev.Type {
	case controlruntime.EventInputAccepted:
		if p, ok := ev.Payload.(controlruntime.MessagePayload); ok {
			card := t.card(ev.RunID)
			card.Title = firstLine(p.Content)
			card.Status = statusAccepted
		}
	case controlruntime.EventRunStarted:
		t.card(ev.RunID).Status = statusRunning
	case controlruntime.EventPhaseChanged:
		if p, ok := ev.Payload.(controlruntime.PhasePayload); ok {
			t.card(ev.RunID).LastPhase = p.Phase
		}
	case controlruntime.EventToolStarted:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			tool := toolCard{Name: p.ToolName, Brief: interaction.ToolBrief(p.ToolName, p.Args), Status: "running"}
			card.Tools = append(card.Tools, tool)
		}
	case controlruntime.EventToolPreview:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			if strings.TrimSpace(p.Preview) == "" || !isDiffLike(p.ToolName, p.Preview) {
				return
			}
			card := t.card(ev.RunID)
			card.Diffs = append(card.Diffs, cleanDiffPreview(p.Preview))
		}
	case controlruntime.EventToolCompleted, controlruntime.EventToolBlocked:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			status := "done"
			if ev.Type == controlruntime.EventToolBlocked || p.IsError {
				status = "blocked"
			}
			t.finishTool(t.card(ev.RunID), p, status)
		}
	case controlruntime.EventApprovalRequested:
		if _, ok := ev.Payload.(controlruntime.ApprovalView); ok {
			t.card(ev.RunID).Status = statusWaitingApproval
		}
	case controlruntime.EventQuestionRequested:
		if _, ok := ev.Payload.(controlruntime.QuestionView); ok {
			t.card(ev.RunID).Status = statusWaitingQuestion
		}
	case controlruntime.EventApprovalResolved, controlruntime.EventQuestionResolved:
		card := t.card(ev.RunID)
		if card.Status == statusWaitingApproval || card.Status == statusWaitingQuestion {
			card.Status = statusRunning
		}
	case controlruntime.EventRunDone:
		if p, ok := ev.Payload.(controlruntime.RunResult); ok {
			card := t.card(ev.RunID)
			card.Status = statusDone
			card.Result = p.Output
		}
	case controlruntime.EventRunFailed:
		if p, ok := ev.Payload.(controlruntime.RunResult); ok {
			card := t.card(ev.RunID)
			card.Status = statusFailed
			card.Result = p.Output
			card.Error = p.Error
		}
	case controlruntime.EventRunCancelled:
		t.card(ev.RunID).Status = statusAborted
	}
}

// DoneReply renders the run's terminal push text: the result body (plus the
// /diff shortcut) for a finished run, or the failure card for a failed one.
// It returns "" when there is nothing worth sending.
func (t *Tracker) DoneReply(runID controlruntime.RunID) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	card := t.runs[runID]
	if card == nil {
		return ""
	}
	return t.doneReplyLocked(card)
}

func (t *Tracker) Status() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.order) == 0 {
		return compactMessage(htmlTitle("状态"), HTMLEscape("空闲"), HTMLEscape("发送消息即可开始任务。"))
	}
	card := t.runs[t.last]
	if card == nil {
		return compactMessage(htmlTitle("状态"), HTMLEscape("空闲"))
	}
	return t.statusSummaryLocked(card)
}

func (t *Tracker) LastSummary() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.order) == 0 {
		return HTMLEscape("还没有远程任务。")
	}
	card := t.runs[t.last]
	if card == nil {
		return HTMLEscape("还没有远程任务。")
	}
	if card.Status == statusDone || card.Status == statusFailed {
		return t.lastSummaryLocked(card)
	}
	return t.cardStatusLocked(card)
}

func (t *Tracker) DiffSummary(runID controlruntime.RunID) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if runID == "" {
		runID = t.last
	}
	card := t.runs[runID]
	if card == nil {
		return HTMLEscape("未找到匹配的任务。")
	}
	if len(card.Diffs) == 0 {
		return HTMLEscape("暂无 diff 预览。")
	}
	return compactMessage(htmlTitle("差异"), htmlPre(connect.TruncateRunes(card.Diffs[len(card.Diffs)-1], 3000)))
}

func (t *Tracker) card(id controlruntime.RunID) *taskCard {
	if id == "" {
		id = "run_unknown"
	}
	if card, ok := t.runs[id]; ok {
		t.last = id
		return card
	}
	card := &taskCard{Status: statusCreated}
	t.runs[id] = card
	t.order = append(t.order, id)
	t.last = id
	if len(t.order) > maxRunCards {
		drop := t.order[0]
		t.order = t.order[1:]
		delete(t.runs, drop)
	}
	return card
}

func (t *Tracker) finishTool(card *taskCard, p controlruntime.ToolPayload, status string) {
	brief := interaction.ToolBrief(p.ToolName, p.Args)
	for i := len(card.Tools) - 1; i >= 0; i-- {
		if card.Tools[i].Name == p.ToolName && card.Tools[i].Status == "running" {
			card.Tools[i].Brief = brief
			card.Tools[i].Status = status
			return
		}
	}
	card.Tools = append(card.Tools, toolCard{
		Name:   p.ToolName,
		Brief:  brief,
		Status: status,
	})
}
