package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func QuotaHook() policy.Hook {
	return policy.Hook{
		Name: "quota", Point: policy.PreModel,
		On: func(s policy.State) *policy.Result {
			left := int64(s.Facts().Turn.ReadsLeft)
			if left > 2 {
				s.SetInt("last_warned", 0)
				return nil
			}
			if left == s.Int("last_warned") {
				return nil
			}
			s.SetInt("last_warned", left)
			sev := "warning"
			content := fmt.Sprintf("剩余 %d 次读取配额。请综合已有证据并推进当前范围内的下一步；不要为了消耗配额继续扫描。", left)
			if left <= 0 {
				sev = "critical"
				content = "读取配额已耗尽。不要再尝试 read/grep/glob；基于已有证据完成当前范围内的工作，或准确说明缺少的关键事实。"
			}
			return &policy.Result{Hint: &policy.Hint{Type: "quota", Severity: sev, Content: content}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("reads_left=%d last_warned=%d", s.Facts().Turn.ReadsLeft, s.Int("last_warned"))
		},
	}
}
