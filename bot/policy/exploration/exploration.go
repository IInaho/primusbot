// Package exploration maintains the exploration score used by policy.
package exploration

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

// Tracker is a concurrency-safe exploration score.
type Tracker struct {
	score atomic.Int64
}

func New() *Tracker {
	t := &Tracker{}
	t.Reset()
	return t
}

func (t *Tracker) Value() int64 {
	if t == nil {
		return 0
	}
	return t.score.Load()
}

func (t *Tracker) Record(toolName string, args map[string]any) {
	if t == nil {
		return
	}
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

func (t *Tracker) Reset() {
	if t != nil {
		t.score.Store(MaxScore)
	}
}

var toolCosts = map[string]int{
	"read":       readCost,
	"grep":       grepCost,
	"glob":       trivialCost,
	"list":       trivialCost,
	"shell":      grepCost,
	"bash":       grepCost,
	"web_search": webSearchCost,
	"web_fetch":  webFetchCost,
	"task":       taskCost,
}

func (t *Tracker) adjust(delta, limit int64) {
	for {
		current := t.score.Load()
		next := current + delta
		if delta < 0 && next < limit {
			next = limit
		}
		if delta > 0 && next > limit {
			next = limit
		}
		if next == current || t.score.CompareAndSwap(current, next) {
			return
		}
	}
}
