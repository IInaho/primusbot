package builtin

import (
	"fmt"

	"nekocode/bot/hooks"
)

func ProgressStallHook() hooks.Hook {
	return hooks.Hook{
		Name: "progress_stall", Point: hooks.PostTool,
		On: func(s hooks.State) *hooks.Result {
			if s.Get(hooks.StoreTurnToolCalls) == 0 {
				return nil
			}
			if s.Get(hooks.StoreHasEdits) == 1 || s.Get(hooks.StoreLedgerProgress) == 1 {
				s.Set(hooks.CounterStallTurns, 0)
				return nil
			}

			n := s.Get(hooks.CounterStallTurns) + 1
			s.Set(hooks.CounterStallTurns, n)
			if n < 8 {
				return nil
			}

			s.Set(hooks.CounterStallTurns, 0)
			return &hooks.Result{
				Hint: &hooks.Hint{Type: "stall", Severity: "warning",
					Content: fmt.Sprintf("连续 %d 轮工具调用没有产生新证据（新文件读取、修改或验证）。基于已有信息推进实际工作，或明确报告阻塞。\n\n你的任务：%s",
						n, s.GetStr(hooks.StoreStepInput))},
			}
		},
		DescribeTrigger: func(s hooks.State) string {
			return fmt.Sprintf("stall_turns=%d ledger_progress=%d", s.Get(hooks.CounterStallTurns), s.Get(hooks.StoreLedgerProgress))
		},
	}
}
