package policy

import (
	"strings"

	hookplugin "nekocode/bot/policy/plugin"
)

func LoadPluginHooks(pluginRoot, hooksPath string) ([]Hook, error) {
	loaded, err := hookplugin.Load(pluginRoot, hooksPath)
	if err != nil {
		return nil, err
	}
	hooks := make([]Hook, 0, len(loaded))
	for _, h := range loaded {
		hook := adaptPluginHook(h)
		hook.Name = "plugin:" + pluginRoot + ":" + strings.TrimPrefix(hook.Name, "plugin:")
		hooks = append(hooks, hook)
	}
	return hooks, nil
}

func adaptPluginHook(h hookplugin.Hook) Hook {
	return Hook{
		Name:  h.Name,
		Point: HookPoint(h.Point),
		On: func(s State) *Result {
			if h.Once {
				if s.Int("started") == 1 {
					return nil
				}
				s.SetInt("started", 1)
			}
			facts := s.Facts()
			result := h.On(hookplugin.Event{
				Tool:  facts.Tool.Name,
				Error: facts.Tool.Error,
			})
			applyPluginState(s, result)
			return adaptPluginResult(result)
		},
	}
}

func applyPluginState(state State, result *hookplugin.Result) {
	if result == nil || result.StatePatch == nil {
		return
	}
	for key, value := range result.StatePatch.Ints {
		state.SetInt(key, value)
	}
	for key, value := range result.StatePatch.Strings {
		state.SetString(key, value)
	}
}

func adaptPluginResult(r *hookplugin.Result) *Result {
	if r == nil {
		return nil
	}
	out := &Result{}
	if r.Hint != nil {
		out.Hint = &Hint{
			Type:     r.Hint.Type,
			Severity: r.Hint.Severity,
			Content:  r.Hint.Content,
		}
	}
	if r.Stop != nil {
		sr := StopReason(r.Stop.Reason)
		out.Stop = &sr
	}
	if r.BlockTool != nil {
		out.BlockTool = &BlockTool{
			Tool:   r.BlockTool.Tool,
			Reason: r.BlockTool.Reason,
		}
	}
	if r.RequireTool != nil {
		out.RequireTool = &RequireTool{
			Tool:   r.RequireTool.Tool,
			Reason: r.RequireTool.Reason,
		}
	}
	if r.BlockFinal != nil {
		out.BlockFinal = &BlockFinal{
			Reason: r.BlockFinal.Reason,
		}
	}
	return out
}
