package catalog

import (
	"testing"

	edittool "nekocode/bot/tools/builtin/filesystem/edit"
	listtool "nekocode/bot/tools/builtin/filesystem/list"
	readtool "nekocode/bot/tools/builtin/filesystem/read"
	searchtool "nekocode/bot/tools/builtin/filesystem/search"
	treetool "nekocode/bot/tools/builtin/filesystem/tree"
	writetool "nekocode/bot/tools/builtin/filesystem/write"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/builtin/tasktool"
	"nekocode/bot/tools/builtin/todo"
	"nekocode/bot/tools/builtin/web"
	"nekocode/bot/tools/runtime/core"
)

// TestInterface verifies Name / Params / ExecutionMode for every tool.
func TestInterface(t *testing.T) {
	tests := []struct {
		tool      core.Tool
		name      string
		mode      core.ExecutionMode
		minParams int
	}{
		{&readtool.ReadTool{}, "read", core.ModeParallel, 3},
		{&writetool.WriteTool{}, "write", core.ModeSequential, 2},
		{&edittool.EditTool{}, "edit", core.ModeSequential, 5},
		{&shell.BashTool{}, "bash", core.ModeSequential, 1},
		{&searchtool.GlobTool{}, "glob", core.ModeParallel, 1},
		{&searchtool.GrepTool{}, "grep", core.ModeParallel, 1},
		{&listtool.ListTool{}, "list", core.ModeParallel, 1},
		{&treetool.TreeTool{}, "tree", core.ModeParallel, 1},
		{&todo.TodoWriteTool{}, "todo_write", core.ModeSequential, 1},
		{tasktool.NewTaskTool(), "task", core.ModeParallel, 1},
		{web.NewWebSearchTool(), "web_search", core.ModeParallel, 1},
		{web.NewWebFetchTool(), "web_fetch", core.ModeParallel, 1},
	}
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
