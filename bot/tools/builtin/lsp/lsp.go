// Package lsp adapts the read-only Language Server Protocol client in
// core (lspcore) to the NekoCode tool interface. It registers four tools —
// lsp_definition, lsp_references, lsp_hover, lsp_diagnostics — that share one
// lazily-spawned server per language, so a whole parallel batch rides a single
// gopls/rust-analyzer/pyright process.
package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	lspcore "nekocode/bot/tools/builtin/lsp/core"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/toolutil"
	"nekocode/bot/tools/runtime/workspace"
)

// LSPTool owns the shared manager for the LSP tool family. The manager is
// created lazily on the first Execute (so registration stays cheap even when
// no language server is installed — a query then returns an install hint) and
// is reused across calls and registry rebuilds. Close shuts down every spawned
// server.
type LSPTool struct {
	mu    sync.Mutex
	specs map[string]lspcore.ServerSpec
	mgr   *lspcore.Manager
}

// NewLSPTool creates an LSP tool family with default language server specs.
func NewLSPTool() *LSPTool {
	return &LSPTool{specs: lspcore.DefaultSpecs()}
}

// NewLSPToolWithSpecs creates an LSP tool family with explicit server specs
// (mainly for tests and for callers that want to override the defaults).
func NewLSPToolWithSpecs(specs map[string]lspcore.ServerSpec) *LSPTool {
	return &LSPTool{specs: specs}
}

// ensureManager returns the shared manager, creating it with the primary
// workspace root on first use.
func (t *LSPTool) ensureManager(ctx context.Context) *lspcore.Manager {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mgr == nil {
		t.mgr = lspcore.NewManager(workspaceRoot(ctx), t.specs)
	}
	return t.mgr
}

// Tools returns the four LSP tools backed by this manager.
func (t *LSPTool) Tools() []core.Tool {
	return []core.Tool{
		&posTool{t, "lsp_definition", "Jump to where a symbol is defined. Give the file, the 1-based line the symbol appears on, and the symbol text itself.", (*lspcore.Manager).Definition},
		&posTool{t, "lsp_references", "List every reference to a symbol across the workspace. Give the file, the 1-based line, and the symbol text.", (*lspcore.Manager).References},
		&posTool{t, "lsp_hover", "Show the type signature and documentation for a symbol. Give the file, the 1-based line, and the symbol text.", (*lspcore.Manager).Hover},
		&diagTool{t},
	}
}

// Close shuts down all spawned language servers. Idempotent.
func (t *LSPTool) Close() {
	t.mu.Lock()
	mgr := t.mgr
	t.mgr = nil
	t.mu.Unlock()
	if mgr != nil {
		mgr.Close()
	}
}

// resolveFile anchors relative paths to the workspace root; absolute paths pass
// through unchanged. LSP queries are read-only and stay outside the sandbox, so
// no workspace boundary check applies — global dependencies (stdlib,
// site-packages, node_modules) outside the project must remain queryable
// without the user granting a permission per path.
func resolveFile(ctx context.Context, file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(workspaceRoot(ctx), file)
}

// workspaceRoot resolves the workspace root the language server should index:
// the first read-write root of the session's workspace manager, falling back to
// the process working directory.
func workspaceRoot(ctx context.Context) string {
	if wm, ok := workspace.FromContext(ctx); ok {
		for _, r := range wm.Snapshot() {
			if r.Access == workspace.AccessReadWrite {
				return r.Path
			}
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

// posTool adapts definition/references/hover: file + line + symbol in, text out.
type posTool struct {
	owner *LSPTool
	name  string
	desc  string
	query func(*lspcore.Manager, context.Context, string, int, string) (string, error)
}

func (t *posTool) Name() string        { return t.name }
func (t *posTool) Description() string { return t.desc }
func (t *posTool) ExecutionMode(map[string]any) core.ExecutionMode {
	return core.ModeParallel
}

func (t *posTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "file", Type: "string", Required: true, Description: "Path to the source file, relative to the workspace root or absolute."},
		{Name: "line", Type: "integer", Required: true, Description: "1-based line number the symbol appears on."},
		{Name: "symbol", Type: "string", Required: true, Description: "The exact symbol text on that line, e.g. \"executeBatch\". Used to locate the column."},
	}
}

func (t *posTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	file, err := toolutil.RequireStringArg(args, "file")
	if err != nil {
		return "", err
	}
	line, err := toolutil.RequireIntArg(args, "line")
	if err != nil {
		return "", err
	}
	if line < 1 {
		return "", fmt.Errorf("line must be >= 1, got %d", line)
	}
	symbol, err := toolutil.RequireStringArg(args, "symbol")
	if err != nil {
		return "", err
	}
	mgr := t.owner.ensureManager(ctx)
	return t.query(mgr, ctx, resolveFile(ctx, file), line, symbol)
}

// diagTool adapts lsp_diagnostics: file in, compiler/linter problems out.
type diagTool struct{ owner *LSPTool }

func (t *diagTool) Name() string { return "lsp_diagnostics" }
func (t *diagTool) Description() string {
	return "Report compiler/linter diagnostics (errors, warnings) for a file from its language server. Use after editing to check the change compiles."
}
func (t *diagTool) ExecutionMode(map[string]any) core.ExecutionMode {
	return core.ModeParallel
}

func (t *diagTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "file", Type: "string", Required: true, Description: "Path to the source file, relative to the workspace root or absolute."},
	}
}

func (t *diagTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	file, err := toolutil.RequireStringArg(args, "file")
	if err != nil {
		return "", err
	}
	mgr := t.owner.ensureManager(ctx)
	return mgr.Diagnostics(ctx, resolveFile(ctx, file))
}
