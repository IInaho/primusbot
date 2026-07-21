package builtin

import (
	"fmt"

	"nekocode/bot/hooks"
)

func VerificationHook() hooks.Hook {
	return hooks.Hook{
		Name: "verification", Point: hooks.PostTurn,
		On: func(s hooks.State) *hooks.Result {
			intent := s.GetStr(hooks.StoreFinalIntent)
			if intent != "" && intent != hooks.FinalIntentFinal && intent != hooks.FinalIntentError {
				return nil
			}
			if s.Get(hooks.StoreHasTasks) == 0 || s.Get(hooks.StoreTasksAllDone) == 1 {
				s.Set(hooks.CounterVerifyInjected, 0)
				return nil
			}
			if s.Get(hooks.StoreTurnToolCalls) > 0 {
				return nil
			}
			if s.Flag(hooks.CounterVerifyInjected) {
				return nil
			}
			s.Set(hooks.CounterVerifyInjected, 1)
			return &hooks.Result{BlockFinal: &hooks.BlockFinal{
				Reason: "你还有未完成的任务，但本轮没有调用任何工具。请继续完成任务；如果只能报告进度，必须明确说明哪些任务未完成。",
			}}
		},
		DescribeTrigger: func(s hooks.State) string {
			return fmt.Sprintf("has_tasks=%d tasks_all_done=%d turn_tool_calls=%d final_intent=%s",
				s.Get(hooks.StoreHasTasks), s.Get(hooks.StoreTasksAllDone), s.Get(hooks.StoreTurnToolCalls),
				dashIfEmpty(s.GetStr(hooks.StoreFinalIntent)))
		},
	}
}
