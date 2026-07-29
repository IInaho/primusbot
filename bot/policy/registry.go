package policy

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

type HookCounts struct {
	Evaluations  int64
	Hints        int64
	Stops        int64
	BlockTools   int64
	RequireTools int64
	BlockFinals  int64
}

func (c HookCounts) String() string {
	return fmt.Sprintf("hooks: %d eval, %d hints, %d stops, %d block_tool, %d require_tool, %d block_final",
		c.Evaluations, c.Hints, c.Stops, c.BlockTools, c.RequireTools, c.BlockFinals)
}

// Registry is the package entry point: it holds the registered hooks and the
// governance store they read and patch, and runs hook evaluations against
// consistent snapshots.
type Registry struct {
	mu      sync.Mutex
	hooks   []Hook
	store   map[string]int64
	strVals map[string]string
	counts  HookCounts
	audit   []HookAuditEvent
	session string
}

func NewRegistry() *Registry {
	return &Registry{
		store:   make(map[string]int64),
		strVals: make(map[string]string),
	}
}

func (r *Registry) Register(h Hook) {
	r.mu.Lock()
	r.hooks = append(r.hooks, h)
	r.mu.Unlock()
}

func (r *Registry) Evaluate(point HookPoint, tool string, toolError bool, toolArgs ...map[string]any) []Result {
	hooks, snap := r.evaluationSnapshot(tool, toolError, toolArgs...)
	results, evaluated, audit := evaluateHooks(hooks, point, snap)
	r.commitEvaluation(results, evaluated, audit, snap.patch)
	return results
}

func (r *Registry) evaluationSnapshot(tool string, toolError bool, toolArgs ...map[string]any) ([]Hook, *Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	snap := &Snapshot{
		Store:   maps.Clone(r.store),
		Tool:    tool,
		Error:   toolError,
		strVals: maps.Clone(r.strVals),
		session: r.session,
	}
	if len(toolArgs) > 0 && toolArgs[0] != nil {
		snap.Args = toolArgs[0]
	}
	return slices.Clone(r.hooks), snap
}

func evaluateHooks(hooks []Hook, point HookPoint, snap *Snapshot) ([]Result, int64, []HookAuditEvent) {
	var results []Result
	var audit []HookAuditEvent
	var evaluated int64

	for _, h := range hooks {
		if h.Point != point {
			continue
		}
		evaluated++
		result := h.On(snap)
		if result == nil {
			continue
		}
		applyResultPatch(snap, result.StatePatch)
		results = append(results, *result)
		event := hookAuditEvent(h, point, snap, result)
		audit = append(audit, event)
		logHookAudit(event)
	}

	return results, evaluated, audit
}

func applyResultPatch(snap *Snapshot, patch *StatePatch) {
	if patch == nil {
		return
	}
	for k, v := range patch.Ints {
		snap.Set(k, v)
	}
	for k, v := range patch.Strings {
		snap.SetStr(k, v)
	}
}

// commitEvaluation records stats and audit from an evaluation and writes the
// accumulated state patch back to the store, under a single lock.
func (r *Registry) commitEvaluation(results []Result, evaluated int64, audit []HookAuditEvent, patch StatePatch) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counts.Evaluations += evaluated
	r.audit = append(r.audit, audit...)
	if len(r.audit) > maxHookAuditEvents {
		r.audit = append([]HookAuditEvent(nil), r.audit[len(r.audit)-maxHookAuditEvents:]...)
	}
	for _, res := range results {
		if res.Hint != nil {
			r.counts.Hints++
		}
		if res.Stop != nil {
			r.counts.Stops++
		}
		if res.BlockTool != nil {
			r.counts.BlockTools++
		}
		if res.RequireTool != nil {
			r.counts.RequireTools++
		}
		if res.BlockFinal != nil {
			r.counts.BlockFinals++
		}
	}

	for k, v := range patch.Ints {
		r.store[k] = v
	}
	for k, v := range patch.Strings {
		r.strVals[k] = v
	}
}

func (r *Registry) ResetSession() {
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.store)
	clear(r.strVals)
	r.counts = HookCounts{}
	r.audit = nil
}

// GovernanceStats formats accumulated hook counts and audit events.
// Side effect: it resets the counters and audit buffer (read-and-reset
// semantics), so each call reports only what happened since the last call.
func (r *Registry) GovernanceStats() string {
	r.mu.Lock()
	c := r.counts
	audit := append([]HookAuditEvent(nil), r.audit...)
	r.counts = HookCounts{}
	r.audit = nil
	r.mu.Unlock()
	if len(audit) == 0 {
		return " | " + c.String()
	}
	return " | " + c.String() + " | hook events: " + formatHookEventSummary(audit) + " | recent hooks: " + formatHookAudit(recentHookAudit(audit, 5))
}

func (r *Registry) hookCountsSnapshot() HookCounts {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts
}

func (r *Registry) hookAuditSnapshot() []HookAuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.audit)
}

func (r *Registry) ResetTurn() {
	r.mu.Lock()
	defer r.mu.Unlock()
	deleteTurnScoped(r.store)
	// Same lifecycle rule as the int store (see keys.go): gauge/value/
	// turn/flag keys are turn-scoped. No gauge/flag str keys exist yet,
	// but strVals follows the same rule to stay consistent.
	deleteTurnScoped(r.strVals)
}

func deleteTurnScoped[V any](m map[string]V) {
	for k := range m {
		if isTurnScopedKey(k) {
			delete(m, k)
		}
	}
}

func isTurnScopedKey(k string) bool {
	return strings.HasPrefix(k, KeyPrefixGauge) ||
		strings.HasPrefix(k, KeyPrefixValue) ||
		strings.HasPrefix(k, KeyPrefixTurn) ||
		strings.HasPrefix(k, KeyPrefixFlag)
}

func (r *Registry) Set(k string, v int64) {
	r.mu.Lock()
	r.store[k] = v
	r.mu.Unlock()
}

func (r *Registry) Inc(k string) {
	r.mu.Lock()
	r.store[k]++
	r.mu.Unlock()
}

func (r *Registry) Flag(k string, v bool) {
	var n int64
	if v {
		n = 1
	}
	r.Set(k, n)
}

func (r *Registry) SetStr(k, v string) {
	r.mu.Lock()
	r.strVals[k] = v
	r.mu.Unlock()
}

func (r *Registry) SetSessionID(id string) {
	r.mu.Lock()
	r.session = id
	r.mu.Unlock()
}

func (r *Registry) List() []Hook {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.hooks)
}

func (r *Registry) UnregisterWhere(fn func(Hook) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := r.hooks[:0]
	for _, h := range r.hooks {
		if !fn(h) {
			filtered = append(filtered, h)
		}
	}
	r.hooks = filtered
}

// StatePatch carries the writes a hook made during one evaluation: applied to
// the evaluating Snapshot immediately (so later hooks see them) and written
// back to the registry store when the evaluation commits.
type StatePatch struct {
	Ints    map[string]int64
	Strings map[string]string
}

func (p *StatePatch) setInt(k string, v int64) {
	if p.Ints == nil {
		p.Ints = make(map[string]int64)
	}
	p.Ints[k] = v
}

func (p *StatePatch) setString(k, v string) {
	if p.Strings == nil {
		p.Strings = make(map[string]string)
	}
	p.Strings[k] = v
}

// Snapshot is the consistent point-in-time view of the store that one
// evaluation runs against: cloned under the registry lock, patched by hooks
// during evaluation, discarded after commit.
type Snapshot struct {
	Store   map[string]int64
	Tool    string
	Args    map[string]any
	Error   bool
	session string
	strVals map[string]string
	patch   StatePatch
}

// State is what a hook sees of the world during evaluation.
type State interface {
	Get(key string) int64
	Set(key string, value int64)
	Flag(key string) bool
	GetStr(key string) string
	ToolName() string
	ToolArgs() map[string]any
	ToolError() bool
}

func (s *Snapshot) Get(key string) int64     { return s.Store[key] }
func (s *Snapshot) Flag(key string) bool     { return s.Store[key] == 1 }
func (s *Snapshot) GetStr(key string) string { return s.strVals[key] }

func (s *Snapshot) Set(key string, value int64) {
	s.Store[key] = value
	s.patch.setInt(key, value)
}

func (s *Snapshot) SetStr(key, v string) {
	s.strVals[key] = v
	s.patch.setString(key, v)
}

func (s *Snapshot) ToolName() string         { return s.Tool }
func (s *Snapshot) ToolArgs() map[string]any { return s.Args }
func (s *Snapshot) ToolError() bool          { return s.Error }
