package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func VerificationHook() policy.Hook {
	return policy.Hook{
		Name: "verification", Point: policy.Stop,
		On: func(s policy.State) *policy.Result {
			facts := s.Facts()
			intent := facts.Response.Intent
			if intent != "" && intent != policy.FinalIntentFinal && intent != policy.FinalIntentError {
				return nil
			}
			if !facts.Turn.HasTasks || facts.Turn.TasksDone {
				s.SetInt("injected", 0)
				return nil
			}
			if facts.Activity.ToolCalls > 0 {
				return nil
			}
			if s.Int("injected") == 1 {
				return nil
			}
			s.SetInt("injected", 1)
			return &policy.Result{BlockFinal: &policy.BlockFinal{
				Reason: "你还有未完成的任务，但本轮没有调用任何工具。请继续完成任务；如果只能报告进度，必须明确说明哪些任务未完成。",
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			facts := s.Facts()
			return fmt.Sprintf("has_tasks=%t tasks_all_done=%t turn_tool_calls=%d final_intent=%s",
				facts.Turn.HasTasks, facts.Turn.TasksDone, facts.Activity.ToolCalls,
				dashIfEmpty(string(facts.Response.Intent)))
		},
	}
}
