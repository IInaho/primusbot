package runstore

import (
	"strings"
	"sync"
	"time"

	"nekocode/runtime/internal/artifact"
	"nekocode/runtime/internal/core"
)

type RunID = core.RunID
type Event = core.Event
type EventType = core.EventType
type RunStatus = core.RunStatus
type RunView = core.RunView
type ToolView = core.ToolView
type ToolStatus = core.ToolStatus
type ArtifactView = core.ArtifactView
type ArtifactItem = core.ArtifactItem
type MessagePayload = core.MessagePayload
type PhasePayload = core.PhasePayload
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload
type ApprovalView = core.ApprovalView
type QuestionView = core.QuestionView

const (
	EventInputAccepted     = core.EventInputAccepted
	EventRunStarted        = core.EventRunStarted
	EventPhaseChanged      = core.EventPhaseChanged
	EventToolStarted       = core.EventToolStarted
	EventToolPreview       = core.EventToolPreview
	EventToolCompleted     = core.EventToolCompleted
	EventToolBlocked       = core.EventToolBlocked
	EventApprovalRequested = core.EventApprovalRequested
	EventApprovalResolved  = core.EventApprovalResolved
	EventQuestionRequested = core.EventQuestionRequested
	EventQuestionResolved  = core.EventQuestionResolved
	EventRunDone           = core.EventRunDone
	EventRunFailed         = core.EventRunFailed
	EventRunAborted        = core.EventRunAborted
	RunRunning             = core.RunRunning
	RunWaitingApproval     = core.RunWaitingApproval
	RunWaitingQuestion     = core.RunWaitingQuestion
	RunDone                = core.RunDone
	RunFailed              = core.RunFailed
	RunAborted             = core.RunAborted
	ToolRunning            = core.ToolRunning
	ToolDone               = core.ToolDone
	ToolBlocked            = core.ToolBlocked
)

const defaultRunStoreLimit = 100

type RunStore struct {
	mu    sync.Mutex
	limit int
	runs  map[RunID]*runRecord
	order []RunID
	last  RunID
}

type runRecord struct {
	view      RunView
	artifact  ArtifactView
	toolStack []int
}

func NewRunStore(limit int) *RunStore {
	if limit <= 0 {
		limit = defaultRunStoreLimit
	}
	return &RunStore{
		limit: limit,
		runs:  make(map[RunID]*runRecord),
	}
}

func (s *RunStore) Record(ev Event) {
	if ev.RunID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := s.record(ev.RunID)
	rec.view.UpdatedAt = ev.Time
	rec.view.EventCount++
	rec.artifact.Events++

	switch ev.Type {
	case EventInputAccepted:
		rec.view.Status = RunRunning
		if rec.view.StartedAt.IsZero() {
			rec.view.StartedAt = ev.Time
		}
		if p, ok := ev.Payload.(MessagePayload); ok {
			rec.view.Input = p.Content
			rec.view.Source = p.Source
			rec.view.Sender = p.Sender
		}
	case EventRunStarted:
		rec.view.Status = RunRunning
		if rec.view.StartedAt.IsZero() {
			rec.view.StartedAt = ev.Time
		}
	case EventPhaseChanged:
		if p, ok := ev.Payload.(PhasePayload); ok {
			rec.view.Phase = p.Phase
		}
	case EventToolStarted:
		if p, ok := ev.Payload.(ToolPayload); ok {
			rec.startTool(p, ev.Time)
		}
	case EventToolPreview:
		if p, ok := ev.Payload.(ToolPayload); ok {
			rec.applyToolPreview(p, ev.Time)
		}
	case EventToolCompleted, EventToolBlocked:
		if p, ok := ev.Payload.(ToolPayload); ok {
			status := ToolDone
			if ev.Type == EventToolBlocked || p.IsError {
				status = ToolBlocked
			}
			rec.finishTool(p, status, ev.Time)
		}
	case EventApprovalRequested:
		if p, ok := ev.Payload.(ApprovalView); ok {
			rec.view.Status = RunWaitingApproval
			rec.upsertApproval(p)
		}
	case EventApprovalResolved:
		if p, ok := ev.Payload.(ApprovalView); ok {
			rec.upsertApproval(p)
		}
		if rec.view.Status == RunWaitingApproval {
			rec.view.Status = RunRunning
		}
	case EventQuestionRequested:
		if p, ok := ev.Payload.(QuestionView); ok {
			rec.view.Status = RunWaitingQuestion
			rec.upsertQuestion(p)
		}
	case EventQuestionResolved:
		if p, ok := ev.Payload.(QuestionView); ok {
			rec.upsertQuestion(p)
		}
		if rec.view.Status == RunWaitingQuestion {
			rec.view.Status = RunRunning
		}
	case EventRunDone:
		if p, ok := ev.Payload.(DonePayload); ok {
			rec.view.Output = p.Output
			if strings.TrimSpace(p.Output) != "" {
				rec.artifact.Results = append(rec.artifact.Results, ArtifactItem{
					Kind:      "result",
					Title:     "Final result",
					Content:   p.Output,
					CreatedAt: ev.Time,
				})
			}
		}
		rec.finish(RunDone, ev.Time)
	case EventRunFailed:
		if p, ok := ev.Payload.(DonePayload); ok {
			rec.view.Output = p.Output
			rec.view.Error = p.Error
			if strings.TrimSpace(p.Output) != "" {
				rec.artifact.Results = append(rec.artifact.Results, ArtifactItem{
					Kind:      "result",
					Title:     "Partial result",
					Content:   p.Output,
					CreatedAt: ev.Time,
				})
			}
			if strings.TrimSpace(p.Error) != "" {
				rec.artifact.Errors = append(rec.artifact.Errors, ArtifactItem{
					Kind:      "error",
					Title:     "Run error",
					Content:   p.Error,
					CreatedAt: ev.Time,
				})
			}
		}
		rec.finish(RunFailed, ev.Time)
	case EventRunAborted:
		if p, ok := ev.Payload.(DonePayload); ok {
			rec.view.Error = p.Error
		}
		rec.finish(RunAborted, ev.Time)
	}
}

func (s *RunStore) CurrentRunView() (RunView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == "" {
		return RunView{}, false
	}
	return s.runViewLocked(s.last)
}

func (s *RunStore) RunView(id RunID) (RunView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runViewLocked(id)
}

func (s *RunStore) ListRunViews(limit int) []RunView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.order) {
		limit = len(s.order)
	}
	out := make([]RunView, 0, limit)
	for i := len(s.order) - 1; i >= 0 && len(out) < limit; i-- {
		if view, ok := s.runViewLocked(s.order[i]); ok {
			out = append(out, view)
		}
	}
	return out
}

func (s *RunStore) ArtifactView(id RunID) (ArtifactView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = s.last
	}
	rec, ok := s.runs[id]
	if !ok {
		return ArtifactView{}, false
	}
	return copyArtifactView(rec.artifact), true
}

func (s *RunStore) record(id RunID) *runRecord {
	if rec, ok := s.runs[id]; ok {
		s.last = id
		return rec
	}
	now := time.Now()
	rec := &runRecord{
		view: RunView{
			ID:        id,
			Status:    RunRunning,
			StartedAt: now,
			UpdatedAt: now,
		},
		artifact: ArtifactView{RunID: id},
	}
	s.runs[id] = rec
	s.order = append(s.order, id)
	s.last = id
	if len(s.order) > s.limit {
		drop := s.order[0]
		s.order = s.order[1:]
		delete(s.runs, drop)
	}
	return rec
}

func (s *RunStore) runViewLocked(id RunID) (RunView, bool) {
	if id == "" {
		id = s.last
	}
	rec, ok := s.runs[id]
	if !ok {
		return RunView{}, false
	}
	return copyRunView(rec.view), true
}

func (r *runRecord) startTool(p ToolPayload, at time.Time) {
	tool := ToolView{
		Name:      p.ToolName,
		Args:      p.Args,
		Preview:   p.Preview,
		Status:    ToolRunning,
		StartedAt: at,
	}
	r.view.Tools = append(r.view.Tools, tool)
	r.toolStack = append(r.toolStack, len(r.view.Tools)-1)
}

func (r *runRecord) applyToolPreview(p ToolPayload, at time.Time) {
	idx := r.findOpenTool(p.ToolName)
	if idx >= 0 {
		r.view.Tools[idx].Preview = p.Preview
	}
	r.recordToolArtifact(p.ToolName, p.Preview, "preview", at)
}

func (r *runRecord) finishTool(p ToolPayload, status ToolStatus, at time.Time) {
	idx := r.findOpenTool(p.ToolName)
	if idx < 0 {
		r.startTool(p, at)
		idx = len(r.view.Tools) - 1
	}
	r.view.Tools[idx].Args = p.Args
	r.view.Tools[idx].Output = p.Output
	r.view.Tools[idx].IsError = p.IsError
	r.view.Tools[idx].Status = status
	r.view.Tools[idx].FinishedAt = &at
	r.removeToolStack(idx)
	if status == ToolDone && !p.IsError {
		r.recordToolArtifact(p.ToolName, p.Output, "result", at)
	}
}

func (r *runRecord) findOpenTool(name string) int {
	for i := len(r.toolStack) - 1; i >= 0; i-- {
		idx := r.toolStack[i]
		if idx >= 0 && idx < len(r.view.Tools) && r.view.Tools[idx].Name == name {
			return idx
		}
	}
	return -1
}

func (r *runRecord) removeToolStack(idx int) {
	for i := len(r.toolStack) - 1; i >= 0; i-- {
		if r.toolStack[i] == idx {
			r.toolStack = append(r.toolStack[:i], r.toolStack[i+1:]...)
			return
		}
	}
}

func (r *runRecord) upsertApproval(view ApprovalView) {
	for i := range r.view.Approvals {
		if r.view.Approvals[i].ID == view.ID {
			r.view.Approvals[i] = view
			return
		}
	}
	r.view.Approvals = append(r.view.Approvals, view)
}

func (r *runRecord) upsertQuestion(view QuestionView) {
	for i := range r.view.Questions {
		if r.view.Questions[i].ID == view.ID {
			r.view.Questions[i] = view
			return
		}
	}
	r.view.Questions = append(r.view.Questions, view)
}

func (r *runRecord) finish(status RunStatus, at time.Time) {
	r.view.Status = status
	r.view.FinishedAt = &at
}

func (r *runRecord) recordToolArtifact(toolName, content, titleSuffix string, at time.Time) {
	classification, ok := artifact.ClassifyToolOutput(toolName, content)
	if !ok {
		return
	}
	item := ArtifactItem{
		Kind:      string(classification.Kind),
		ToolName:  toolName,
		Title:     toolName + " " + titleSuffix,
		Content:   content,
		CreatedAt: at,
	}
	switch classification.Kind {
	case artifact.KindPatch:
		r.artifact.Patches = append(r.artifact.Patches, item)
	case artifact.KindReview:
		r.artifact.Reviews = append(r.artifact.Reviews, item)
	case artifact.KindDiff:
		r.artifact.Diffs = append(r.artifact.Diffs, item)
	}
}

func copyRunView(in RunView) RunView {
	out := in
	out.Tools = append([]ToolView(nil), in.Tools...)
	out.Approvals = append([]ApprovalView(nil), in.Approvals...)
	out.Questions = append([]QuestionView(nil), in.Questions...)
	return out
}

func copyArtifactView(in ArtifactView) ArtifactView {
	out := in
	out.Diffs = append([]ArtifactItem(nil), in.Diffs...)
	out.Patches = append([]ArtifactItem(nil), in.Patches...)
	out.Reviews = append([]ArtifactItem(nil), in.Reviews...)
	out.Results = append([]ArtifactItem(nil), in.Results...)
	out.Errors = append([]ArtifactItem(nil), in.Errors...)
	return out
}
