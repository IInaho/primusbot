package catalog

import (
	"testing"

	edittool "nekocode/bot/tools/builtin/filesystem/edit"
	listtool "nekocode/bot/tools/builtin/filesystem/list"
	readtool "nekocode/bot/tools/builtin/filesystem/read"
	searchtool "nekocode/bot/tools/builtin/filesystem/search"
	treetool "nekocode/bot/tools/builtin/filesystem/tree"
	writetool "nekocode/bot/tools/builtin/filesystem/write"
	"nekocode/bot/tools/builtin/lsp"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/builtin/task"
	"nekocode/bot/tools/builtin/todo"
	"nekocode/bot/tools/builtin/web"
	"nekocode/bot/tools/runtime/core"
)

// TestInterface verifies Name / Params / ExecutionMode for every tool.
func TestInterface(t *testing.T) {
	lspTools := lsp.NewLSPTool().Tools()
	lspCases := make([]struct {
		tool      core.Tool
		name      string
		mode      core.ExecutionMode
		minParams int
	}, 0, 4)
	for _, tt := range lspTools {
		lspCases = append(lspCases, struct {
			tool      core.Tool
			name      string
			mode      core.ExecutionMode
			minParams int
		}{tt, tt.Name(), core.ModeParallel, 1})
	}
	tests := []struct {
		tool      core.Tool
		name      string
		mode      core.ExecutionMode
		minParams int
	}{
		{&readtool.ReadTool{}, "read", core.ModeParallel, 3},
		{&writetool.WriteTool{}, "write", core.ModeSequential, 2},
		{&edittool.EditTool{}, "edit", core.ModeSequential, 5},
		{&shell.ShellTool{}, "shell", core.ModeSequential, 5},
		{shell.NewProcessTool(&shell.ShellTool{}), "process", core.ModeSequential, 3},
		{&searchtool.GlobTool{}, "glob", core.ModeParallel, 1},
		{&searchtool.GrepTool{}, "grep", core.ModeParallel, 1},
		{&listtool.ListTool{}, "list", core.ModeParallel, 1},
		{&treetool.TreeTool{}, "tree", core.ModeParallel, 1},
		{&todo.TodoWriteTool{}, "todo_write", core.ModeSequential, 1},
		{task.NewTaskTool(), "task", core.ModeParallel, 1},
		{web.NewWebSearchTool(), "web_search", core.ModeParallel, 1},
		{web.NewWebFetchTool(), "web_fetch", core.ModeParallel, 1},
	}
	tests = append(tests, lspCases...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool.Name() != tt.name {
				t.Errorf("Name() = %q, want %q", tt.tool.Name(), tt.name)
			}
			if tt.tool.ExecutionMode(nil) != tt.mode {
				t.Errorf("ExecutionMode = %v, want %v", tt.tool.ExecutionMode(nil), tt.mode)
			}
			if n := len(tt.tool.Parameters()); n < tt.minParams {
				t.Errorf("Parameters() = %d, want >= %d", n, tt.minParams)
			}
			if tt.tool.Description() == "" {
				t.Error("Description() is empty")
			}
		})
	}
}
