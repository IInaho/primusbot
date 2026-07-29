package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func ProgressStallHook() policy.Hook {
	return policy.Hook{
		Name: "progress_stall", Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			facts := s.Facts()
			if facts.Activity.ToolCalls == 0 {
				return nil
			}
			if facts.Activity.HasEdits || facts.Activity.HasProgress {
				s.SetInt("stall_turns", 0)
				return nil
			}

			n := s.Int("stall_turns") + 1
			s.SetInt("stall_turns", n)
			if n < 8 {
				return nil
			}

			s.SetInt("stall_turns", 0)
			return &policy.Result{
				Hint: &policy.Hint{Type: "stall", Severity: "warning",
					Content: fmt.Sprintf("连续 %d 轮工具调用没有产生新证据（新文件读取、修改或验证）。基于已有信息推进实际工作，或明确报告阻塞。\n\n你的任务：%s",
						n, facts.Turn.Input)},
			}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("stall_turns=%d has_progress=%t", s.Int("stall_turns"), s.Facts().Activity.HasProgress)
		},
	}
}
