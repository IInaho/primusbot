package builder

import (
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/provider/types"
)

type Builder struct {
	State *state.State
}

func (b *Builder) Build() []types.Message {
	b.State.Mu.RLock()
	defer b.State.Mu.RUnlock()
	out := b.State.Ctx.BuildLayer0()
	out = append(out, b.State.Ctx.BuildLayer0Mem()...)
	out = append(out, b.State.Ctx.BuildLayer05()...)
	out = append(out, b.filterValidMessages(b.visibleHistory())...)
	out = append(out, b.State.Ctx.BuildLayer2()...)

	for i := range out {
		out[i].Source = ""
	}
	return out
}

func (b *Builder) visibleHistory() []types.Message {
	if b.State.Ctx.CompactBoundary <= 0 {
		return b.State.Ctx.Messages
	}
	if b.State.Ctx.CompactBoundary >= len(b.State.Ctx.Messages) {
		return nil
	}
	return b.State.Ctx.Messages[b.State.Ctx.CompactBoundary:]
}

func (b *Builder) filterValidMessages(kept []types.Message) []types.Message {
	hasResult := map[string]bool{}
	for _, msg := range kept {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			hasResult[msg.ToolCallID] = true
		}
	}
	validAsst := map[int]bool{}
	for i, msg := range kept {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			ok := true
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" && !hasResult[tc.ID] {
					ok = false
					break
				}
			}
			if ok {
				validAsst[i] = true
			}
		}
	}
	validIDs := map[string]bool{}
	filtered := make([]types.Message, 0, len(kept))
	for i, msg := range kept {
		if msg.Content == "" && msg.Role != "system" {
			msg.Content = "."
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			if !validAsst[i] {
				continue
			}
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					validIDs[tc.ID] = true
				}
			}
		}
		if msg.Role == "tool" && (msg.ToolCallID == "" || !validIDs[msg.ToolCallID]) {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}
