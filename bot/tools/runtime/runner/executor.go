package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/common"
)

// ToolRegistry is the minimal registry contract required by the executor.
type ToolRegistry interface {
	Get(name string) (core.Tool, error)
}

// Previewer is an optional interface for tools that can generate a preview
// before execution.
type Previewer interface {
	Preview(args map[string]any) string
}

type Executor struct {
	registry  ToolRegistry
	state     *execution.ExecutionState
	confirmFn common.ConfirmFunc
	phaseFn   common.PhaseFunc
	planMode  bool
	previewFn func(toolName string, args map[string]any, preview string)
	permStore *permission.Store
	fnMu      sync.RWMutex
	// escalationApproved tracks tool calls (keyed by ToolCallItem.ID) for
	// which the user pre-approved permission escalation in the merged confirm
	// dialog. Keying by call ID — not tool name — prevents a pre-approval
	// granted for one call from being silently spent by a later, unrelated
	// call to the same tool (which used to let a one-time "allow & escalate"
	// on an innocent ls bypass the second dialog for a process.host call).
	escalationMu       sync.Mutex
	escalationApproved map[string]escalationApproval
	// Permission rule engine (claude-code style allow/ask/deny). The engine's
	// deny→ask→allow decision is the single authority for whether a tool call
	// runs, prompts, or is blocked.
	permEngine    *permission.Engine
	permDecl      permission.PermissionsDecl
	permWorkspace string
	permHome      string

	// sessionGrants holds in-memory capability grants created by one-time
	// ("仅本次允许并授权") approvals. They live for the current process only
	// and are never written to disk — the retry path queries them via
	// permissionAllowed, but closing NekoCode forgets them. Remembered
	// ("始终允许并授权") approvals go through rememberPermission → permStore
	// and additionally register here for the live session.
	sessionGrantsMu sync.RWMutex
	sessionGrants   []sessionGrant
}

// sessionGrant is an in-memory capability grant scoped to the running process.
type sessionGrant struct {
	tool         string
	capabilities []string
	writePaths   []string
}

func NewExecutor(r ToolRegistry) *Executor {
	root, _ := os.Getwd()
	e := &Executor{
		registry:           r,
		state:              execution.NewExecutionState(),
		permStore:          permission.NewStore(root),
		escalationApproved: make(map[string]escalationApproval),
	}
	e.rebuildEngine(permission.PermissionsDecl{}, e.permStore, root)
	return e
}

func (e *Executor) ExecutionState() *execution.ExecutionState { return e.state }

func (e *Executor) SetConfirmFn(fn common.ConfirmFunc) {
	e.fnMu.Lock()
	e.confirmFn = fn
	e.fnMu.Unlock()
}

func (e *Executor) ConfirmFn() common.ConfirmFunc {
	e.fnMu.RLock()
	defer e.fnMu.RUnlock()
	return e.confirmFn
}

func (e *Executor) SetPhaseFn(fn common.PhaseFunc) {
	e.fnMu.Lock()
	e.phaseFn = fn
	e.fnMu.Unlock()
}

func (e *Executor) SetPlanMode(on bool) {
	e.fnMu.Lock()
	e.planMode = on
	e.fnMu.Unlock()
}

func (e *Executor) SetPreviewFn(fn func(string, map[string]any, string)) {
	e.fnMu.Lock()
	e.previewFn = fn
	e.fnMu.Unlock()
}

func (e *Executor) SetPermissionStore(store *permission.Store) {
	e.fnMu.Lock()
	e.permStore = store
	e.fnMu.Unlock()
}

// SetProjectStore binds the executor to a per-project permissions file at
// <project>/.nekocode/permissions.json. After this, the engine is rebuilt
// against that project's grants/rules. Subsequent Allow/Remember calls
// target that project file (no more cross-project leakage).
func (e *Executor) SetProjectStore(projectRoot string) {
	projectRoot = filepath.Clean(projectRoot)
	e.fnMu.Lock()
	e.permStore = permission.NewStore(projectRoot)
	e.permWorkspace = projectRoot
	decl := e.permDecl
	store := e.permStore
	e.fnMu.Unlock()
	eng, err := permission.NewEngineForWorkspace(decl, store, projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "permission: %v — blocking tool calls until config is fixed\n", err)
		eng = permission.NewEngine(permission.DefaultMatchers())
		eng.SetRules([]permission.Rule{{Tool: "*", Effect: permission.EffectDeny, Source: "config-error"}})
		e.fnMu.Lock()
		e.permEngine = eng
		e.fnMu.Unlock()
		return
	}
	e.fnMu.Lock()
	e.permEngine = eng
	e.fnMu.Unlock()
}

// SetPermissionPolicy configures the declarative permission rule engine.
// executeOne evaluates every tool call through this engine (deny→ask→allow).
// workspace/home are used to resolve path anchors in file-tool rules.
func (e *Executor) SetPermissionPolicy(decl permission.PermissionsDecl, workspace, home string) {
	e.fnMu.Lock()
	e.permDecl = decl
	e.permWorkspace = workspace
	e.permHome = home
	store := e.permStore
	e.fnMu.Unlock()
	e.rebuildEngine(decl, store, workspace)
}

// SetWorkspace updates the workspace used for path-anchor resolution and
// rebuilds the engine (e.g. after /cd). Safe to call when no policy is set.
func (e *Executor) SetWorkspace(workspace, home string) {
	e.fnMu.Lock()
	decl := e.permDecl
	e.fnMu.Unlock()
	if !hasPermissionDecl(decl) {
		e.fnMu.Lock()
		e.permWorkspace = workspace
		e.permHome = home
		e.fnMu.Unlock()
		return
	}
	e.SetPermissionPolicy(decl, workspace, home)
}

func hasPermissionDecl(decl permission.PermissionsDecl) bool {
	return len(decl.Allow)+len(decl.Ask)+len(decl.Deny)+len(decl.Sandbox) > 0
}

// rebuildEngine reconstructs the permission engine from decl + store +
// workspace. Malformed declared rules fail closed so a bad config cannot
// silently disable the permission engine.
func (e *Executor) rebuildEngine(decl permission.PermissionsDecl, store *permission.Store, workspace string) {
	eng, err := permission.NewEngineForWorkspace(decl, store, workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "permission: %v — blocking tool calls until config is fixed\n", err)
		eng = permission.NewEngine(permission.DefaultMatchers())
		eng.SetRules([]permission.Rule{{Tool: "*", Effect: permission.EffectDeny, Source: "config-error"}})
		e.fnMu.Lock()
		e.permEngine = eng
		e.fnMu.Unlock()
		return
	}
	e.fnMu.Lock()
	e.permEngine = eng
	e.fnMu.Unlock()
}

type escalationApproval struct {
	remember bool
}

// preApproveEscalation marks a tool call (by its ID) as having user
// pre-approved permission escalation in the merged confirm dialog, so
// tryPermissionEscalation will skip the second dialog. Keying by call ID
// prevents pre-approvals from leaking to unrelated later calls.
func (e *Executor) preApproveEscalation(callID string, remember bool) {
	if callID == "" {
		return
	}
	e.escalationMu.Lock()
	e.escalationApproved[callID] = escalationApproval{remember: remember}
	e.escalationMu.Unlock()
}

// escalationPreApproved checks (and clears) the pre-approval flag for a tool
// call. Returns (approval, true) if a pre-approval was set for this call ID.
func (e *Executor) escalationPreApproved(callID string) (escalationApproval, bool) {
	if callID == "" {
		return escalationApproval{}, false
	}
	e.escalationMu.Lock()
	defer e.escalationMu.Unlock()
	if approval, ok := e.escalationApproved[callID]; ok {
		delete(e.escalationApproved, callID)
		return approval, true
	}
	return escalationApproval{}, false
}

// dropEscalationApproval unconditionally clears any pre-approval outstanding
// for callID. Called from executeOne on every terminal path so a successful
// "allow & escalate" click never leaves a stale token waiting to be spent by
// some later call that happens to reuse the ID.
func (e *Executor) dropEscalationApproval(callID string) {
	if callID == "" {
		return
	}
	e.escalationMu.Lock()
	delete(e.escalationApproved, callID)
	e.escalationMu.Unlock()
}

// addSessionGrant records an in-memory capability grant for the current
// process. One-time approvals use this so the retry path finds the grant
// without anything being persisted to disk. CapProcessHost is never stored
// — every host execution must prompt, by design.
func (e *Executor) addSessionGrant(toolName string, req core.PermissionRequest) {
	if toolName == "" || len(req.Capabilities) == 0 {
		return
	}
	for _, c := range req.Capabilities {
		if c == core.CapProcessHost {
			return
		}
	}
	caps := append([]string(nil), req.Capabilities...)
	writePaths := permission.WritePathsFromRequest(req)
	e.sessionGrantsMu.Lock()
	defer e.sessionGrantsMu.Unlock()
	for _, g := range e.sessionGrants {
		if g.tool == toolName && slices.Equal(g.capabilities, caps) && slices.Equal(g.writePaths, writePaths) {
			return
		}
	}
	e.sessionGrants = append(e.sessionGrants, sessionGrant{tool: toolName, capabilities: caps, writePaths: writePaths})
}

// matchSessionGrant reports whether an in-memory session grant covers the
// requested capabilities for the tool.
func (e *Executor) matchSessionGrant(toolName string, req core.PermissionRequest) bool {
	if len(req.Capabilities) == 0 {
		return false
	}
	e.sessionGrantsMu.RLock()
	defer e.sessionGrantsMu.RUnlock()
	for _, g := range e.sessionGrants {
		if g.tool != toolName {
			continue
		}
		if sessionGrantMatches(g, req) {
			return true
		}
	}
	return false
}

func sessionGrantMatches(g sessionGrant, req core.PermissionRequest) bool {
	for _, need := range req.Capabilities {
		found := false
		for _, have := range g.capabilities {
			if have == need {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if !slices.Contains(req.Capabilities, core.CapFsWritePath) {
		return true
	}
	return permission.ContainsAllWritePaths(g.writePaths, permission.WritePathsFromRequest(req))
}
