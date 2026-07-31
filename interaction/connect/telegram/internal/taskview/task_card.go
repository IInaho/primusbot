package taskview

import (
	"fmt"
	"strings"
	"sync"

	"nekocode/interaction"
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
	mu               sync.Mutex
	runs             map[controlruntime.RunID]*taskCard
	order            []controlruntime.RunID
	last             controlruntime.RunID
	pendingQuestions map[string]pendingQuestion
	lastQuestionID   string
}

type pendingQuestion struct {
	View controlruntime.QuestionView
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
		runs:             make(map[controlruntime.RunID]*taskCard),
		pendingQuestions: make(map[string]pendingQuestion),
	}
}

func (t *Tracker) RenderEvent(ev controlruntime.Event) string {
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
		card := t.card(ev.RunID)
		card.Status = statusRunning
	case controlruntime.EventPhaseChanged:
		if p, ok := ev.Payload.(controlruntime.PhasePayload); ok {
			card := t.card(ev.RunID)
			card.LastPhase = p.Phase
		}
	case controlruntime.EventToolStarted:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			tool := toolCard{Name: p.ToolName, Brief: interaction.ToolBrief(p.ToolName, p.Args), Status: "running"}
			card.Tools = append(card.Tools, tool)
		}
	case controlruntime.EventToolPreview:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			if strings.TrimSpace(p.Preview) == "" {
				return ""
			}
			if isDiffLike(p.ToolName, p.Preview) {
				clean := cleanDiffPreview(p.Preview)
				card.Diffs = append(card.Diffs, clean)
				return renderDiffPreview(p.Preview)
			}
		}
	case controlruntime.EventToolCompleted, controlruntime.EventToolBlocked:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			status := "done"
			if ev.Type == controlruntime.EventToolBlocked || p.IsError {
				status = "blocked"
			}
			t.finishTool(card, p, status)
			if status == "blocked" {
				return compactMessage(
					htmlTitle("已阻止"),
					labelCode("工具", strings.TrimSpace(p.ToolName+" "+interaction.ToolBrief(p.ToolName, p.Args))),
					htmlPre(truncateRunes(p.Output, 1200)),
				)
			}
		}
	case controlruntime.EventApprovalRequested:
		if p, ok := ev.Payload.(controlruntime.ApprovalView); ok {
			card := t.card(ev.RunID)
			card.Status = statusWaitingApproval
			return compactMessage(htmlTitle("需要审批"), approvalSummary(p))
		}
	case controlruntime.EventQuestionRequested:
		if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
			card := t.card(ev.RunID)
			card.Status = statusWaitingQuestion
			t.pendingQuestions[p.ID] = pendingQuestion{View: p}
			t.lastQuestionID = p.ID
			return compactMessage(htmlTitle("提问"), questionSummary(p))
		}
	case controlruntime.EventApprovalResolved, controlruntime.EventQuestionResolved:
		card := t.card(ev.RunID)
		if card.Status == statusWaitingApproval || card.Status == statusWaitingQuestion {
			card.Status = statusRunning
		}
		if ev.Type == controlruntime.EventQuestionResolved {
			if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
				delete(t.pendingQuestions, p.ID)
			}
		}
	case controlruntime.EventRunDone:
		if p, ok := ev.Payload.(controlruntime.RunResult); ok {
			card := t.card(ev.RunID)
			card.Status = statusDone
			card.Result = p.Output
			return t.doneReplyLocked(card)
		}
	case controlruntime.EventRunFailed:
		if p, ok := ev.Payload.(controlruntime.RunResult); ok {
			card := t.card(ev.RunID)
			card.Status = statusFailed
			card.Result = p.Output
			card.Error = p.Error
			return t.doneReplyLocked(card)
		}
	case controlruntime.EventRunCancelled:
		card := t.card(ev.RunID)
		card.Status = statusAborted
		return htmlTitle("已停止")
	}
	return ""
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
	return compactMessage(htmlTitle("差异"), htmlPre(truncateRunes(card.Diffs[len(card.Diffs)-1], 3000)))
}

func (t *Tracker) BuildQuestionReply(questionID, raw string) (controlruntime.QuestionReply, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if strings.TrimSpace(questionID) == "" {
		questionID = t.lastQuestionID
	}
	pending, ok := t.pendingQuestions[questionID]
	if !ok {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("question %s is not pending", questionID)
	}
	reply, err := buildQuestionReply(pending.View, raw)
	if err != nil {
		return controlruntime.QuestionReply{}, "", err
	}
	return reply, questionID, nil
}

func (t *Tracker) RejectQuestion(questionID string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if strings.TrimSpace(questionID) == "" {
		questionID = t.lastQuestionID
	}
	if _, ok := t.pendingQuestions[questionID]; !ok {
		return "", fmt.Errorf("question %s is not pending", questionID)
	}
	return questionID, nil
}

func (t *Tracker) BuildQuestionOptionReply(questionID string, optionIndex int) (controlruntime.QuestionReply, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pending, ok := t.pendingQuestions[questionID]
	if !ok {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("question %s is not pending", questionID)
	}
	if len(pending.View.Questions) != 1 {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("question %s requires text answer", questionID)
	}
	item := pending.View.Questions[0]
	if item.Multiple || optionIndex < 0 || optionIndex >= len(item.Options) {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("question %s requires text answer", questionID)
	}
	return controlruntime.QuestionReply{Answers: [][]string{{item.Options[optionIndex].Label}}}, questionID, nil
}

// PendingQuestion returns the pending question view for interactive
// (button-driven) answering.
func (t *Tracker) PendingQuestion(questionID string) (controlruntime.QuestionView, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pending, ok := t.pendingQuestions[questionID]
	return pending.View, ok
}

// BuildQuestionMultiOptionReply builds a reply for a Multiple question from
// the selected option indices. At least one index is required.
func (t *Tracker) BuildQuestionMultiOptionReply(questionID string, indices []int) (controlruntime.QuestionReply, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pending, ok := t.pendingQuestions[questionID]
	if !ok {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("question %s is not pending", questionID)
	}
	if len(pending.View.Questions) != 1 || !pending.View.Questions[0].Multiple {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("question %s is not a multi-select question", questionID)
	}
	item := pending.View.Questions[0]
	var labels []string
	for i, opt := range item.Options {
		for _, idx := range indices {
			if idx == i {
				labels = append(labels, opt.Label)
			}
		}
	}
	if len(labels) == 0 {
		return controlruntime.QuestionReply{}, "", fmt.Errorf("请至少选择一项")
	}
	return controlruntime.QuestionReply{Answers: [][]string{labels}}, questionID, nil
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
