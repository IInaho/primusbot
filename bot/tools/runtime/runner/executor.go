package runner

import (
	"fmt"
	"os"
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
	// escalationApproved tracks tool names for which the user pre-approved
	// permission escalation in the merged confirm dialog.
	escalationMu       sync.Mutex
	escalationApproved map[string]bool
	// Permission rule engine (claude-code style allow/ask/deny). The engine's
	// deny→ask→allow decision is the single authority for whether a tool call
	// runs, prompts, or is blocked.
	permEngine    *permission.Engine
	permDecl      permission.PermissionsDecl
	permWorkspace string
	permHome      string
}

func NewExecutor(r ToolRegistry) *Executor {
	e := &Executor{
		registry:           r,
		state:              execution.NewExecutionState(),
		permStore:          permission.DefaultStore(),
		escalationApproved: make(map[string]bool),
	}
	e.rebuildEngine(permission.PermissionsDecl{}, e.permStore, "")
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
	if len(decl.Allow)+len(decl.Ask)+len(decl.Deny) == 0 {
		e.fnMu.Lock()
		e.permWorkspace = workspace
		e.permHome = home
		e.fnMu.Unlock()
		return
	}
	e.SetPermissionPolicy(decl, workspace, home)
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

// preApproveEscalation marks a tool as having user pre-approved permission
// escalation, so tryPermissionEscalation will skip the second dialog.
func (e *Executor) preApproveEscalation(toolName string) {
	e.escalationMu.Lock()
	e.escalationApproved[toolName] = true
	e.escalationMu.Unlock()
}

// escalationPreApproved checks (and clears) the pre-approval flag for a tool.
func (e *Executor) escalationPreApproved(toolName string) bool {
	e.escalationMu.Lock()
	defer e.escalationMu.Unlock()
	if e.escalationApproved[toolName] {
		delete(e.escalationApproved, toolName)
		return true
	}
	return false
}
