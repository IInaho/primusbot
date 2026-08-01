package catalog

import (
	"nekocode/bot/config"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/diff"
	"nekocode/bot/tools/builtin/filesystem/edit"
	"nekocode/bot/tools/builtin/filesystem/list"
	"nekocode/bot/tools/builtin/filesystem/read"
	"nekocode/bot/tools/builtin/filesystem/search"
	"nekocode/bot/tools/builtin/filesystem/tree"
	"nekocode/bot/tools/builtin/filesystem/write"
	"nekocode/bot/tools/builtin/index"
	"nekocode/bot/tools/builtin/media"
	"nekocode/bot/tools/builtin/question"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/builtin/task"
	"nekocode/bot/tools/builtin/todo"
	"nekocode/bot/tools/builtin/web"
)

func registerAll(r *tools.Registry, imageGenModels []config.ImageGenConfig, shellTool *shell.ShellTool) {
	r.Register(shellTool)
	r.Register(shell.NewProcessTool(shellTool))
	r.Register(&read.ReadTool{})
	r.Register(&write.WriteTool{})
	r.Register(&list.ListTool{})
	r.Register(&tree.TreeTool{})
	r.Register(&search.GlobTool{})
	r.Register(&edit.EditTool{})
	r.Register(&search.GrepTool{})
	r.Register(web.NewWebSearchTool())
	r.Register(web.NewWebFetchTool())
	r.Register(question.NewTool())
	r.Register(&todo.TodoWriteTool{})
	r.Register(task.NewTaskTool())
	r.Register(diff.NewTool())
	r.Register(index.NewIndexTool())

	if len(imageGenModels) > 0 {
		r.Register(media.NewImageGenTool(imageGenModels))
	}
	r.AllowInPlan("read", "grep", "glob", "list", "tree", "web_search", "web_fetch")
}
