package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

const (
	toolResultThreshold = 40
	toolResultInterval  = 10
)

func ToolResultGuardrailHook() policy.Hook {
	return policy.Hook{
		Name: "tool_result_guardrail", Point: policy.PreModelRequest,
		On: func(s policy.State) *policy.Result {
			count := int64(s.Facts().Model.ToolResults)
			lastWarned := s.Int("last_warned")
			if count <= toolResultThreshold || count-lastWarned < toolResultInterval {
				return nil
			}
			s.SetInt("last_warned", count)
			return &policy.Result{Hint: &policy.Hint{
				Type:     "tool_results",
				Severity: "warning",
				Content:  fmt.Sprintf("%d tool results accumulated. Check for unfinished sub-tasks - if any, continue with task. If all done, call task(verify) to validate, then report results.", count),
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("tool_results=%d last_warned=%d threshold=%d interval=%d",
				s.Facts().Model.ToolResults, s.Int("last_warned"),
				toolResultThreshold, toolResultInterval)
		},
	}
}

func ReadBeforeWriteHook() policy.Hook {
	return policy.Hook{
		Name: "read_before_write", Point: policy.PreToolUse,
		On: func(s policy.State) *policy.Result {
			tool := s.Facts().Tool
			name := tool.Name
			if name != "edit" && name != "write" {
				return nil
			}
			path := tool.TargetPath
			if path == "" || !tool.TargetExists || tool.TargetWasRead {
				return nil
			}
			if name == "edit" && tool.EditAnchorSufficient {
				return nil
			}
			return &policy.Result{BlockTool: &policy.BlockTool{
				Tool:   name,
				Reason: "你正在修改 " + path + "，但 ledger 中没有该文件的读取记录。请先 Read 确认当前内容，确认差异后再 edit/write。",
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			tool := s.Facts().Tool
			return fmt.Sprintf("target=%s exists=%t was_read=%t anchor_sufficient=%t",
				dashIfEmpty(tool.TargetPath), tool.TargetExists, tool.TargetWasRead, tool.EditAnchorSufficient)
		},
	}
}

func ReadOnlySpiralHook() policy.Hook {
	return policy.Hook{
		Name: "read_only_spiral", Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			streak := s.Facts().Activity.ReadOnlyStreak
			hint := policy.ReadOnlySpiralHint(streak)
			if hint == nil {
				s.SetInt("last_warned", 0)
				return nil
			}
			if int64(streak)-s.Int("last_warned") < policy.ReadOnlySpiralThreshold {
				return nil
			}
			s.SetInt("last_warned", int64(streak))
			return &policy.Result{Hint: hint}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("read_only_streak=%d", s.Facts().Activity.ReadOnlyStreak)
		},
	}
}
