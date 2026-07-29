package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func ProgressStallHook() policy.Hook {
	return policy.Hook{
		Name: "progress_stall", Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			if s.Get(policy.StoreTurnToolCalls) == 0 {
				return nil
			}
			if s.Get(policy.StoreHasEdits) == 1 || s.Get(policy.StoreLedgerProgress) == 1 {
				s.Set(policy.CounterStallTurns, 0)
				return nil
			}

			n := s.Get(policy.CounterStallTurns) + 1
			s.Set(policy.CounterStallTurns, n)
			if n < 8 {
				return nil
			}

			s.Set(policy.CounterStallTurns, 0)
			return &policy.Result{
				Hint: &policy.Hint{Type: "stall", Severity: "warning",
					Content: fmt.Sprintf("连续 %d 轮工具调用没有产生新证据（新文件读取、修改或验证）。基于已有信息推进实际工作，或明确报告阻塞。\n\n你的任务：%s",
						n, s.GetStr(policy.StoreStepInput))},
			}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("stall_turns=%d ledger_progress=%d", s.Get(policy.CounterStallTurns), s.Get(policy.StoreLedgerProgress))
		},
	}
}
