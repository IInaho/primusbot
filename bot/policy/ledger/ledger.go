// Package ledger collects tool events and exposes run and turn snapshots.
package ledger

import (
	"fmt"
	"sync"

	"nekocode/bot/policy/semantics"
)

// ToolEvent is one classified tool outcome.
type ToolEvent struct {
	Name      string
	Args      map[string]any
	Output    string
	Error     string
	Blocked   bool
	BlockText string
	Semantics semantics.Semantics
}

type Verification struct {
	Command     string
	Passed      bool
	Trusted     bool
	ProjectRule bool
	Output      string
}

// Ledger owns session read history, run audit evidence and current-turn facts.
type Ledger struct {
	mu sync.RWMutex

	readFiles      pathSet
	modifiedFiles  pathSet
	blockedTools   []ToolEvent
	toolErrors     []ToolEvent
	verifications  []Verification
	toolEventCount int
	exploreCalls   int

	turnToolCalls       int
	turnResearcherCalls int
	turnHasEdits        bool
	turnHasProgress     bool
}

// New creates an empty ledger.
func New() *Ledger {
	return &Ledger{
		readFiles:     make(pathSet),
		modifiedFiles: make(pathSet),
	}
}

// ResetRun clears run evidence while preserving session read history.
func (l *Ledger) ResetRun() {
	l.mu.Lock()
	defer l.mu.Unlock()
	// readFiles is intentionally preserved across runs: once the LLM has read a
	// file in this session, we trust it to edit that file without re-reading.
	// All run and turn evidence is cleared below.
	l.modifiedFiles = make(pathSet)
	l.blockedTools = nil
	l.toolErrors = nil
	l.verifications = nil
	l.toolEventCount = 0
	l.exploreCalls = 0
	l.resetTurn()
}

// BeginTurn clears only facts scoped to one model turn. Session reads and
// run-level audit evidence remain available.
func (l *Ledger) BeginTurn() {
	l.mu.Lock()
	l.resetTurn()
	l.mu.Unlock()
}

func (l *Ledger) resetTurn() {
	l.turnToolCalls = 0
	l.turnResearcherCalls = 0
	l.turnHasEdits = false
	l.turnHasProgress = false
}

func (l *Ledger) RecordTool(ev ToolEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.toolEventCount++
	l.turnToolCalls++
	if ev.Semantics.Exploratory {
		l.exploreCalls++
	}
	if ev.Name == "task" {
		if kind, _ := ev.Args["type"].(string); kind == "researcher" {
			l.turnResearcherCalls++
		}
	}
	if ev.Blocked {
		l.blockedTools = append(l.blockedTools, ev)
		l.turnHasProgress = true
		return
	}
	if ev.Error != "" {
		l.toolErrors = append(l.toolErrors, ev)
		l.turnHasProgress = true
	}
	if ev.Error == "" && ev.Semantics.SourceProducing {
		l.readFiles.addAll(extractReadPaths(ev.Name, ev.Args))
		l.turnHasProgress = true
	}
	if ev.Semantics.Mutating {
		modified := extractModifiedPaths(ev)
		l.modifiedFiles.addAll(modified)
		if len(modified) > 0 {
			l.turnHasEdits = true
			l.turnHasProgress = true
		}
		if ev.Name == "write" {
			l.readFiles.addAll(modified)
		}
	}
	if ev.Semantics.Verifying {
		l.verifications = append(l.verifications, Verification{
			Command:     commandArg(ev.Args),
			Passed:      ev.Error == "",
			Trusted:     ev.Semantics.VerificationTrusted,
			ProjectRule: ev.Semantics.VerificationProjectRule,
			Output:      ev.Output,
		})
		l.turnHasProgress = true
	}
}

func (l *Ledger) Snapshot() Snapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s := Snapshot{
		ReadFiles:     l.readFiles.sorted(),
		ModifiedFiles: l.modifiedFiles.sorted(),
	}
	s.BlockedTools = append(s.BlockedTools, l.blockedTools...)
	s.ToolErrors = append(s.ToolErrors, l.toolErrors...)
	s.Verifications = append(s.Verifications, l.verifications...)
	s.ToolEventCount = l.toolEventCount
	return s
}

func (l *Ledger) Restore(s Snapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.readFiles = newPathSetFrom(s.ReadFiles)
	l.modifiedFiles = newPathSetFrom(s.ModifiedFiles)
	l.blockedTools = append([]ToolEvent(nil), s.BlockedTools...)
	l.toolErrors = append([]ToolEvent(nil), s.ToolErrors...)
	l.verifications = append([]Verification(nil), s.Verifications...)
	l.toolEventCount = s.ToolEventCount
	l.resetTurn()
}

// Snapshot is the persisted ledger state.
type Snapshot struct {
	ReadFiles      []string
	ModifiedFiles  []string
	BlockedTools   []ToolEvent
	ToolErrors     []ToolEvent
	Verifications  []Verification
	ToolEventCount int
}

// TurnSnapshot contains only facts used to decide the current turn.
type TurnSnapshot struct {
	ToolCalls       int
	ExploreCalls    int
	ResearcherCalls int
	HasEdits        bool
	HasProgress     bool
}

func (l *Ledger) TurnSnapshot() TurnSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return TurnSnapshot{
		ToolCalls:       l.turnToolCalls,
		ExploreCalls:    l.exploreCalls,
		ResearcherCalls: l.turnResearcherCalls,
		HasEdits:        l.turnHasEdits,
		HasProgress:     l.turnHasProgress,
	}
}

// WasRead checks whether a specific file path has been read (tracked in ledger).
// The path is cleaned before comparison to match ledger storage format.
func (l *Ledger) WasRead(path string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.readFiles.has(path)
}

func (s Snapshot) HasModifications() bool {
	return len(s.ModifiedFiles) > 0
}

func (s Snapshot) HasPassingVerification() bool {
	for _, v := range s.Verifications {
		if v.Passed {
			return true
		}
	}
	return false
}

func (s Snapshot) Summary() string {
	return fmt.Sprintf("%d modified, %d verifications, %d tool errors, %d blocked tools",
		len(s.ModifiedFiles), len(s.Verifications), len(s.ToolErrors), len(s.BlockedTools))
}
