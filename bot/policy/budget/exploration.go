package budget

import (
	"sync/atomic"

	"nekocode/bot/policy/semantics"
)

const (
	MaxScore      = 200
	editRestore   = 60
	readCost      = 5
	grepCost      = 3
	webSearchCost = 3
	webFetchCost  = 8
	taskCost      = 12
	trivialCost   = 2
)

// ExplorationTracker implements a decay-score mechanism:
// starts at 200, tools deduct, edits restore.
// When score <= 0, forced precipitation is triggered via PreTurn hook.
//
// Score is atomic: the main agent and concurrent subagents share one
// tracker through policy.Policy.
type ExplorationTracker struct {
	Score atomic.Int64
}

// NewExplorationTracker creates a fresh tracker at max score.
func NewExplorationTracker() *ExplorationTracker {
	t := &ExplorationTracker{}
	t.Score.Store(MaxScore)
	return t
}

// RecordCall updates the exploration budget based on the tool called.
func (t *ExplorationTracker) RecordCall(toolName string, args map[string]any) {
	sem := semantics.ClassifyToolCall(toolName, args)
	if sem.Mutating {
		t.adjust(editRestore, MaxScore)
		return
	}
	if !sem.Exploratory {
		return
	}
	if cost, ok := toolCosts[toolName]; ok {
		t.adjust(int64(-cost), 0)
	}
}

// Reset fully restores the exploration budget.
func (t *ExplorationTracker) Reset() {
	t.Score.Store(MaxScore)
}

// toolCosts maps exploration tools to their score deduction.
var toolCosts = map[string]int{
	"read":       readCost,
	"grep":       grepCost,
	"glob":       trivialCost,
	"list":       trivialCost,
	"shell":      grepCost, // shell can be exploratory (ls, cat, etc.)
	"bash":       grepCost, // legacy histories before shell merge.
	"web_search": webSearchCost,
	"web_fetch":  webFetchCost,
	"task":       taskCost,
}

// adjust moves the score by delta (negative to deduct), clamped to limit
// (floor for deductions, ceiling for restores).
func (t *ExplorationTracker) adjust(delta, limit int64) {
	for {
		cur := t.Score.Load()
		next := cur + delta
		if delta < 0 && next < limit {
			next = limit
		}
		if delta > 0 && next > limit {
			next = limit
		}
		if next == cur || t.Score.CompareAndSwap(cur, next) {
			return
		}
	}
}
