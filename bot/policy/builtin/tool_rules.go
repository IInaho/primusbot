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
			count := s.Get(policy.StoreToolResultCount)
			lastWarned := s.Get(policy.CounterToolResultWarned)
			if count <= toolResultThreshold || count-lastWarned < toolResultInterval {
				return nil
			}
			s.Set(policy.CounterToolResultWarned, count)
			return &policy.Result{Hint: &policy.Hint{
				Type:     "tool_results",
				Severity: "warning",
				Content:  fmt.Sprintf("%d tool results accumulated. Check for unfinished sub-tasks - if any, continue with task. If all done, call task(verify) to validate, then report results.", count),
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("tool_results=%d last_warned=%d threshold=%d interval=%d",
				s.Get(policy.StoreToolResultCount), s.Get(policy.CounterToolResultWarned),
				toolResultThreshold, toolResultInterval)
		},
	}
}

func ReadBeforeWriteHook() policy.Hook {
	return policy.Hook{
		Name: "read_before_write", Point: policy.PreToolUse,
		On: func(s policy.State) *policy.Result {
			name := s.ToolName()
			if name != "edit" && name != "write" {
				return nil
			}
			path := s.GetStr(policy.StoreEditTargetPath)
			if path == "" || s.Get(policy.StoreEditTargetExists) != 1 || s.Get(policy.StoreEditTargetWasRead) == 1 {
				return nil
			}
			if name == "edit" && s.Get(policy.StoreEditAnchorSufficient) == 1 {
				return nil
			}
			return &policy.Result{BlockTool: &policy.BlockTool{
				Tool:   name,
				Reason: "你正在修改 " + path + "，但 ledger 中没有该文件的读取记录。请先 Read 确认当前内容，确认差异后再 edit/write。",
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("target=%s exists=%d was_read=%d anchor_sufficient=%d",
				dashIfEmpty(s.GetStr(policy.StoreEditTargetPath)),
				s.Get(policy.StoreEditTargetExists),
				s.Get(policy.StoreEditTargetWasRead),
				s.Get(policy.StoreEditAnchorSufficient))
		},
	}
}

const ReadOnlySpiralThreshold = 3

// ReadOnlySpiralHint returns the read-only-spiral warning once streak reaches
// the threshold, nil otherwise. It backs ReadOnlySpiralHook and is also used
// directly by subagents, which track streaks in their own run state rather
// than in the shared registry.
func ReadOnlySpiralHint(streak int) *policy.Hint {
	if streak < ReadOnlySpiralThreshold {
		return nil
	}
	return &policy.Hint{
		Type:     "read_only_spiral",
		Severity: "warning",
		Content:  "You've been reading without acting. Summarize your findings now - don't read any more files.",
	}
}

func ReadOnlySpiralHook() policy.Hook {
	return policy.Hook{
		Name: "read_only_spiral", Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			hint := ReadOnlySpiralHint(int(s.Get(policy.StoreReadOnlyStreak)))
			if hint == nil {
				return nil
			}
			s.Set(policy.StoreReadOnlyStreak, 0)
			return &policy.Result{Hint: hint}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("read_only_streak=%d", s.Get(policy.StoreReadOnlyStreak))
		},
	}
}
