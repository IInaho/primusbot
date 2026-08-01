package runner

import (
	"context"

	"nekocode/bot/tools/runtime/core"
)

// PreparePreviews prepares previews with the registry's default authority.
func (e *Executor) PreparePreviews(calls []core.ToolCallItem) {
	e.PreparePreviewsContext(context.Background(), calls)
}

// PreparePreviewsContext stores previews using request-scoped authority.
func (e *Executor) PreparePreviewsContext(ctx context.Context, calls []core.ToolCallItem) {
	ctx = e.toolContext(ctx)
	for i, c := range calls {
		if calls[i].Args == nil {
			calls[i].Args = map[string]any{}
			c.Args = calls[i].Args
		}
		if t, err := e.registry.Get(c.Name); err == nil {
			if preview, ok := previewTool(ctx, t, c.Args); ok {
				calls[i].Args["_preview"] = preview
			}
		}
	}
}

func (e *Executor) emitPreview(ctx context.Context, call core.ToolCallItem) {
	if t, err := e.registry.Get(call.Name); err == nil {
		if _, contextAware := t.(ContextPreviewer); contextAware {
			e.emitToolPreview(ctx, call, t)
		} else if _, legacy := t.(Previewer); legacy {
			e.emitToolPreview(ctx, call, t)
		}
	}
}

func (e *Executor) emitToolPreview(ctx context.Context, call core.ToolCallItem, tool core.Tool) {
	if call.Args == nil {
		call.Args = map[string]any{}
	}
	preview, _ := call.Args["_preview"].(string)
	if preview == "" {
		preview, _ = previewTool(e.toolContext(ctx), tool, call.Args)
		call.Args["_preview"] = preview
	}
	e.fnMu.RLock()
	pfn := e.previewFn
	e.fnMu.RUnlock()
	if pfn != nil {
		pfn(call.Name, call.Args, preview)
	}
}

func previewTool(ctx context.Context, tool core.Tool, args map[string]any) (string, bool) {
	if previewer, ok := tool.(ContextPreviewer); ok {
		return previewer.PreviewContext(ctx, args), true
	}
	if previewer, ok := tool.(Previewer); ok {
		return previewer.Preview(args), true
	}
	return "", false
}
