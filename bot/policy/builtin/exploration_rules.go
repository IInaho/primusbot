package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func ExplorationExhaustedHook() policy.Hook {
	return policy.Hook{
		Name: "exploration_exhausted", Point: policy.PreTurn,
		On: func(s policy.State) *policy.Result {
			facts := s.Facts()
			if facts.Activity.ExploreCalls < 10 {
				s.SetInt("injected", 0)
				return nil
			}
			if facts.Exploration.Score > 0 {
				s.SetInt("injected", 0)
				return nil
			}
			if s.Int("injected") == 1 {
				return nil
			}
			s.SetInt("injected", 1)
			return &policy.Result{
				Hint: &policy.Hint{Type: "exploration", Severity: "warning",
					Content: "你已经探索较多。优先基于已有信息推进实际工作；只有缺少关键事实时才继续探索。\n\n你的任务：" + facts.Turn.Input},
			}
		},
		DescribeTrigger: func(s policy.State) string {
			facts := s.Facts()
			return fmt.Sprintf("explore_calls=%d score=%d has_edits=%t",
				facts.Activity.ExploreCalls, facts.Exploration.Score, facts.Activity.HasEdits)
		},
	}
}

func ExploreCascadeHook() policy.Hook {
	return policy.Hook{
		Name: "explore_cascade", Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			facts := s.Facts()
			n := facts.Activity.ResearcherCalls
			if n < 4 {
				return nil
			}
			return &policy.Result{Hint: &policy.Hint{Type: "explore_cascade", Severity: "warning",
				Content: fmt.Sprintf("你已经启动了 %d 个 researcher 子 Agent。如果已收集足够信息，立即综合发现并行动。\n\n你的任务：%s",
					n, facts.Turn.Input)}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("researcher_calls=%d", s.Facts().Activity.ResearcherCalls)
		},
	}
}
