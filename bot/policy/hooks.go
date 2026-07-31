package policy

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

type hookCounts struct {
	Evaluations  int64
	Hints        int64
	Stops        int64
	BlockTools   int64
	RequireTools int64
	BlockFinals  int64
}

func (c hookCounts) String() string {
	return fmt.Sprintf("hooks: %d eval, %d hints, %d stops, %d block_tool, %d require_tool, %d block_final",
		c.Evaluations, c.Hints, c.Stops, c.BlockTools, c.RequireTools, c.BlockFinals)
}

// hookEngine owns hook registration, hook-private state and audit output.
// Policy is its only production caller.
type hookEngine struct {
	evalMu  sync.Mutex
	mu      sync.Mutex
	hooks   []Hook
	state   map[string]hookMemory
	counts  hookCounts
	audit   []hookAudit
	session string
}

type hookMemory struct {
	ints    map[string]int64
	strings map[string]string
}

func newHookEngine() *hookEngine {
	return &hookEngine{state: make(map[string]hookMemory)}
}

func (e *hookEngine) register(h Hook) {
	if h.Name == "" || h.On == nil {
		return
	}
	e.mu.Lock()
	for i := range e.hooks {
		if e.hooks[i].Name == h.Name {
			e.hooks[i] = h
			delete(e.state, h.Name)
			e.mu.Unlock()
			return
		}
	}
	e.hooks = append(e.hooks, h)
	e.mu.Unlock()
}

func (e *hookEngine) evaluate(point HookPoint, facts Facts) []Result {
	e.evalMu.Lock()
	defer e.evalMu.Unlock()

	e.mu.Lock()
	hooks := slices.Clone(e.hooks)
	session := e.session
	e.mu.Unlock()

	var results []Result
	for _, h := range hooks {
		if h.Point != point {
			continue
		}
		e.mu.Lock()
		e.counts.Evaluations++
		state := e.stateFor(h.Name, facts)
		e.mu.Unlock()

		result := h.On(state)

		e.mu.Lock()
		e.saveState(h.Name, state)
		if result == nil {
			e.mu.Unlock()
			continue
		}
		results = append(results, *result)
		e.count(result)
		event := hookAuditEvent(h, point, state, result, session)
		e.audit = append(e.audit, event)
		e.mu.Unlock()
		logHookAudit(event)
	}

	e.mu.Lock()
	if len(e.audit) > maxHookAuditEvents {
		e.audit = append([]hookAudit(nil), e.audit[len(e.audit)-maxHookAuditEvents:]...)
	}
	e.mu.Unlock()
	return results
}

func (e *hookEngine) stateFor(name string, facts Facts) *hookState {
	memory := e.state[name]
	return &hookState{
		facts:   cloneFacts(facts),
		ints:    maps.Clone(memory.ints),
		strings: maps.Clone(memory.strings),
	}
}

func cloneFacts(facts Facts) Facts {
	facts.Tool.Args = maps.Clone(facts.Tool.Args)
	return facts
}

func (e *hookEngine) saveState(name string, state *hookState) {
	e.state[name] = hookMemory{
		ints:    maps.Clone(state.ints),
		strings: maps.Clone(state.strings),
	}
}

func (e *hookEngine) count(result *Result) {
	if result.Hint != nil {
		e.counts.Hints++
	}
	if result.Stop != nil {
		e.counts.Stops++
	}
	if result.BlockTool != nil {
		e.counts.BlockTools++
	}
	if result.RequireTool != nil {
		e.counts.RequireTools++
	}
	if result.BlockFinal != nil {
		e.counts.BlockFinals++
	}
}

func (e *hookEngine) reset() {
	e.evalMu.Lock()
	defer e.evalMu.Unlock()
	e.mu.Lock()
	clear(e.state)
	e.counts = hookCounts{}
	e.audit = nil
	e.mu.Unlock()
}

func (e *hookEngine) summary() string {
	e.mu.Lock()
	counts := e.counts
	audit := slices.Clone(e.audit)
	e.mu.Unlock()

	if len(audit) == 0 {
		return " | " + counts.String()
	}
	return " | " + counts.String() + " | hook events: " +
		formatHookEventSummary(audit) + " | recent hooks: " +
		formatHookAudit(recentHookAudit(audit, 5))
}

func (e *hookEngine) setSessionID(id string) {
	e.mu.Lock()
	e.session = id
	e.mu.Unlock()
}

func (e *hookEngine) unregisterPrefix(prefix string) {
	if prefix == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	filtered := e.hooks[:0]
	for _, hook := range e.hooks {
		if !strings.HasPrefix(hook.Name, prefix) {
			filtered = append(filtered, hook)
			continue
		}
		delete(e.state, hook.Name)
	}
	e.hooks = filtered
}

type hookState struct {
	facts   Facts
	ints    map[string]int64
	strings map[string]string
}

func (s *hookState) Facts() Facts {
	return s.facts
}

func (s *hookState) Int(name string) int64 {
	return s.ints[name]
}

func (s *hookState) SetInt(name string, value int64) {
	if s.ints == nil {
		s.ints = make(map[string]int64)
	}
	s.ints[name] = value
}

func (s *hookState) String(name string) string {
	return s.strings[name]
}

func (s *hookState) SetString(name, value string) {
	if s.strings == nil {
		s.strings = make(map[string]string)
	}
	s.strings[name] = value
}
