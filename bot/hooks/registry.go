package hooks

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"nekocode/common/debug"
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

type HookAuditEvent struct {
	Session string
	Hook    string
	Point   HookPoint
	Tool    string
	Action  string
	Detail  string
	Trigger string
}

func (e HookAuditEvent) String() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s@%s:%s", e.Hook, e.Point, e.Action)
	}
	return fmt.Sprintf("%s@%s:%s(%s)", e.Hook, e.Point, e.Action, truncateAuditDetail(e.Detail))
}

const maxHookAuditEvents = 64

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
	r.recordEvaluation(results, evaluated, audit)
	r.applyPatch(snap.patch)
	return results
}

func (r *Registry) evaluationSnapshot(tool string, toolError bool, toolArgs ...map[string]any) ([]Hook, *Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hooks := make([]Hook, len(r.hooks))
	copy(hooks, r.hooks)

	storeCopy := make(map[string]int64, len(r.store))
	for k, v := range r.store {
		storeCopy[k] = v
	}
	strCopy := make(map[string]string, len(r.strVals))
	for k, v := range r.strVals {
		strCopy[k] = v
	}

	snap := &Snapshot{Store: storeCopy, Tool: tool, Error: toolError, strVals: strCopy, session: r.session}
	if len(toolArgs) > 0 && toolArgs[0] != nil {
		snap.Args = toolArgs[0]
	}
	return hooks, snap
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
		snap.set(k, v)
	}
	for k, v := range patch.Strings {
		snap.setStr(k, v)
	}
}

func hookAuditEvent(h Hook, point HookPoint, snap *Snapshot, result *Result) HookAuditEvent {
	event := HookAuditEvent{
		Session: snap.session,
		Hook:    h.Name,
		Point:   point,
		Tool:    snap.Tool,
		Trigger: hookTrigger(h, snap),
	}
	switch {
	case result.Stop != nil:
		event.Action = "stop"
		event.Detail = result.Stop.String()
	case result.BlockTool != nil:
		event.Action = "block_tool"
		event.Detail = result.BlockTool.Reason
	case result.RequireTool != nil:
		event.Action = "require_tool"
		event.Detail = result.RequireTool.Reason
	case result.BlockFinal != nil:
		event.Action = "block_final"
		event.Detail = result.BlockFinal.Reason
	case result.Hint != nil:
		event.Action = "hint"
		event.Detail = result.Hint.Type
		if result.Hint.Severity != "" {
			event.Detail += "/" + result.Hint.Severity
		}
	default:
		event.Action = "state_patch"
	}
	return event
}

func logHookAudit(event HookAuditEvent) {
	debug.Log("hook_audit session=%s point=%s hook=%s action=%s tool=%s trigger={%s} detail=%q",
		emptyAsDash(event.Session), event.Point, event.Hook, event.Action, emptyAsDash(event.Tool), event.Trigger, event.Detail)
}

func emptyAsDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// hookTrigger renders the audit trigger context for a fired hook. Hooks that
// carry a DescribeTrigger describe themselves (keeping trigger details and
// thresholds next to the hook logic); anything else falls back to tool args.
func hookTrigger(h Hook, snap *Snapshot) string {
	if h.DescribeTrigger != nil {
		return h.DescribeTrigger(snap)
	}
	return formatToolArgs(snap.Args)
}

func quoteArg(v any) string {
	s, ok := v.(string)
	if !ok || s == "" {
		return "-"
	}
	return strconv.Quote(truncateAuditDetail(s))
}

func formatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return "args=-"
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+quoteArg(args[k]))
	}
	return "args=" + strings.Join(parts, ",")
}

func (r *Registry) recordEvaluation(results []Result, evaluated int64, audit []HookAuditEvent) {
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
}

func (r *Registry) applyPatch(patch StatePatch) {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	for k := range r.store {
		delete(r.store, k)
	}
	for k := range r.strVals {
		delete(r.strVals, k)
	}
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

func (r *Registry) HookCountsSnapshot() HookCounts {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts
}

func (r *Registry) HookAuditSnapshot() []HookAuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]HookAuditEvent, len(r.audit))
	copy(out, r.audit)
	return out
}

func recentHookAudit(events []HookAuditEvent, n int) []HookAuditEvent {
	if len(events) <= n {
		out := make([]HookAuditEvent, len(events))
		copy(out, events)
		return out
	}
	out := make([]HookAuditEvent, n)
	copy(out, events[len(events)-n:])
	return out
}

func formatHookAudit(events []HookAuditEvent) string {
	parts := make([]string, 0, len(events))
	for _, event := range events {
		parts = append(parts, event.String())
	}
	return strings.Join(parts, ", ")
}

func formatHookEventSummary(events []HookAuditEvent) string {
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, event := range events {
		key := event.Action + ":" + event.Hook
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
	}
	parts := make([]string, 0, len(order))
	for _, key := range order {
		n := counts[key]
		label := strings.Replace(key, ":", "=", 1)
		if n > 1 {
			label = fmt.Sprintf("%s×%d", label, n)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func truncateAuditDetail(s string) string {
	const max = 80
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "..."
}

func (r *Registry) ResetTurn() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.store {
		if isTurnScopedKey(k) {
			delete(r.store, k)
		}
	}
	for k := range r.strVals {
		// Same lifecycle rule as the int store (see keys.go): gauge/value/
		// turn/flag keys are turn-scoped. No gauge/flag str keys exist yet,
		// but strVals follows the same rule to stay consistent.
		if isTurnScopedKey(k) {
			delete(r.strVals, k)
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
	out := make([]Hook, len(r.hooks))
	copy(out, r.hooks)
	return out
}

func (r *Registry) Unregister(name string) {
	r.UnregisterWhere(func(h Hook) bool { return h.Name == name })
}

func (r *Registry) UnregisterWhere(fn func(Hook) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := make([]Hook, 0, len(r.hooks))
	for _, h := range r.hooks {
		if !fn(h) {
			filtered = append(filtered, h)
		}
	}
	r.hooks = filtered
}
