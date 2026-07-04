package builtin

import (
	"fmt"

	"nekocode/bot/hooks"
)

func QuotaHook() hooks.Hook {
	return hooks.Hook{
		Name: "quota", Point: hooks.PreTurn,
		On: func(s hooks.State) *hooks.Result {
			left := s.Get(hooks.StoreQuotaReads)
			if left > 2 {
				s.Set(hooks.CounterQuotaWarned, 0)
				return nil
			}
			if left == s.Get(hooks.CounterQuotaWarned) {
				return nil
			}
			s.Set(hooks.CounterQuotaWarned, left)
			sev := "warning"
			content := fmt.Sprintf("剩余 %d 次读取配额。请使用已有信息，优先进行实质性修改。", left)
			if left <= 0 {
				sev = "critical"
				content = "读取配额已耗尽。不要再尝试 read/grep/glob——基于已有信息行动。"
			}
			return &hooks.Result{Hint: &hooks.Hint{Type: "quota", Severity: sev, Content: content}}
		},
	}
}
