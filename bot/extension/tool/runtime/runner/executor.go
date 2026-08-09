package runner

import (
	"fmt"
	"nekocode/protocol"
	"os"
	"path/filepath"
	"slices"
	"sync"

	tools "nekocode/bot/extension/tool"
	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/execution"
	"nekocode/bot/extension/tool/runtime/permission"
	"nekocode/bot/extension/tool/runtime/workspace"
)

type Executor struct {
	registry  *tools.Registry
	workspace *workspace.Manager
	state     *execution.ExecutionState
	confirmFn protocol.ConfirmFunc
	phaseFn   protocol.PhaseFunc
	planMode  bool
	// fullAccess is the "全接管" (full-takeover) permission mode: every tool
	// call runs without approval prompts. Explicit deny rules still block —
	// a deny is a hard user-configured rule, not an approval.
	fullAccess bool
	planTools  map[string]struct{}
	previewFn  func(toolName string, args map[string]any, preview string)
	permStore  *permission.Store
	fnMu       sync.RWMutex
	// Permission rule engine (claude-code style allow/ask/deny). The engine's
	// deny→ask→allow decision is the single authority for whether a tool call
	// runs, prompts, or is blocked.
	permEngine    *permission.Engine
	permDecl      permission.PermissionsDecl
	permWorkspace string
	permHome      string
}

func NewExecutor(r *tools.Registry) *Executor {
	root, _ := os.Getwd()
	manager := workspace.New(root, nil)
	if r != nil && r.Workspace() != nil {
		manager = r.Workspace()
	}
	e := &Executor{
		registry:      r,
		workspace:     manager,
		state:         execution.NewExecutionState(),
		permStore:     permission.NewStore(root),
		permWorkspace: root,
	}
	e.rebuildEngine(permission.PermissionsDecl{}, e.permStore, root)
	return e
}

func (e *Executor) ExecutionState() *execution.ExecutionState { return e.state }

func (e *Executor) SetConfirmFn(fn protocol.ConfirmFunc) {
	e.fnMu.Lock()
	e.confirmFn = fn
	e.fnMu.Unlock()
}

func (e *Executor) ConfirmFn() protocol.ConfirmFunc {
	e.fnMu.RLock()
	defer e.fnMu.RUnlock()
	return e.confirmFn
}

func (e *Executor) SetPhaseFn(fn protocol.PhaseFunc) {
	e.fnMu.Lock()
	e.phaseFn = fn
	e.fnMu.Unlock()
}

func (e *Executor) SetPlanMode(on bool) {
	e.fnMu.Lock()
	e.planMode = on
	e.fnMu.Unlock()
}

// SetFullAccess toggles the full-takeover permission mode. When on, every
// approval prompt (permission asks and capability escalations) is bypassed
// and the call runs immediately; explicit deny rules still block.
func (e *Executor) SetFullAccess(on bool) {
	e.fnMu.Lock()
	e.fullAccess = on
	e.fnMu.Unlock()
}

// FullAccess reports whether the full-takeover permission mode is active.
func (e *Executor) FullAccess() bool {
	e.fnMu.RLock()
	defer e.fnMu.RUnlock()
	return e.fullAccess
}

// SetPlanTools replaces the tools allowed while plan mode is active. Registry
// declarations are used when this method is not called.
func (e *Executor) SetPlanTools(names ...string) {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name != "" {
			allowed[name] = struct{}{}
		}
	}
	e.fnMu.Lock()
	e.planTools = allowed
	e.fnMu.Unlock()
}

func (e *Executor) planAllows(name string, registered bool) bool {
	e.fnMu.RLock()
	allowed := e.planTools
	_, configured := allowed[name]
	e.fnMu.RUnlock()
	if allowed != nil {
		return configured
	}
	return registered
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

// SandboxEngine returns the current permission engine (or nil if not ready).
// Tools use it to look up sandbox rules (e.g. pnpm dev → network).
func (e *Executor) SandboxEngine() *permission.Engine {
	e.fnMu.Lock()
	defer e.fnMu.Unlock()
	return e.permEngine
}

type escalationApproval struct {
	remember bool
	// predicted is the capability request included in the unified dialog.
	// The pre-approval only covers escalation requests within this scope —
	// see requestCoveredBy.
	predicted *core.PermissionRequest
}

// requestCoveredBy reports whether an actual escalation request stays within
// the capabilities the user saw and approved. Without this check a tool
// could request broader capabilities at execution time (e.g. process.host
// when only net.outbound was predicted) and get them silently.
func (a escalationApproval) requestCoveredBy(req core.PermissionRequest) bool {
	if a.predicted == nil {
		return false
	}
	for _, cap := range req.Capabilities {
		if !slices.Contains(a.predicted.Capabilities, cap) {
			return false
		}
	}
	return permission.ContainsAllWritePaths(
		permission.WritePathsFromRequest(*a.predicted),
		permission.WritePathsFromRequest(req),
	)
}
