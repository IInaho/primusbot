package catalog

import (
	"nekocode/bot/config"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/diff"
	edittool "nekocode/bot/tools/builtin/filesystem/edit"
	listtool "nekocode/bot/tools/builtin/filesystem/list"
	readtool "nekocode/bot/tools/builtin/filesystem/read"
	searchtool "nekocode/bot/tools/builtin/filesystem/search"
	treetool "nekocode/bot/tools/builtin/filesystem/tree"
	writetool "nekocode/bot/tools/builtin/filesystem/write"
	"nekocode/bot/tools/builtin/media"
	"nekocode/bot/tools/builtin/question"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/builtin/tasktool"
	"nekocode/bot/tools/builtin/todo"
	"nekocode/bot/tools/builtin/web"
)

func RegisterAll(r *tools.Registry, imageGenModels []config.ImageGenConfig) {
	r.Register(&shell.BashTool{})
	r.Register(&shell.BgTool{})
	r.Register(&readtool.ReadTool{})
	r.Register(&writetool.WriteTool{})
	r.Register(&listtool.ListTool{})
	r.Register(&treetool.TreeTool{})
	r.Register(&searchtool.GlobTool{})
	r.Register(&edittool.EditTool{})
	r.Register(&searchtool.GrepTool{})
	r.Register(web.NewWebSearchTool())
	r.Register(web.NewWebFetchTool())
	r.Register(question.NewTool())
	r.Register(&todo.TodoWriteTool{})
	r.Register(tasktool.NewTaskTool())
	r.Register(diff.NewTool())

	if len(imageGenModels) > 0 {
		r.Register(media.NewImageGenTool(imageGenModels))
	}
}
