package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func VerificationHook() policy.Hook {
	return policy.Hook{
		Name: "verification", Point: policy.PostTurn,
		On: func(s policy.State) *policy.Result {
			intent := s.GetStr(policy.StoreFinalIntent)
			if intent != "" && intent != policy.FinalIntentFinal && intent != policy.FinalIntentError {
				return nil
			}
			if s.Get(policy.StoreHasTasks) == 0 || s.Get(policy.StoreTasksAllDone) == 1 {
				s.Set(policy.CounterVerifyInjected, 0)
				return nil
			}
			if s.Get(policy.StoreTurnToolCalls) > 0 {
				return nil
			}
			if s.Flag(policy.CounterVerifyInjected) {
				return nil
			}
			s.Set(policy.CounterVerifyInjected, 1)
			return &policy.Result{BlockFinal: &policy.BlockFinal{
				Reason: "你还有未完成的任务，但本轮没有调用任何工具。请继续完成任务；如果只能报告进度，必须明确说明哪些任务未完成。",
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("has_tasks=%d tasks_all_done=%d turn_tool_calls=%d final_intent=%s",
				s.Get(policy.StoreHasTasks), s.Get(policy.StoreTasksAllDone), s.Get(policy.StoreTurnToolCalls),
				dashIfEmpty(s.GetStr(policy.StoreFinalIntent)))
		},
	}
}
