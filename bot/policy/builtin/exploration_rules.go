package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func ExplorationExhaustedHook() policy.Hook {
	return policy.Hook{
		Name: "exploration_exhausted", Point: policy.PreTurn,
		On: func(s policy.State) *policy.Result {
			if s.Get(policy.StoreExploreCalls) < 10 {
				s.Set(policy.CounterExploreInjected, 0)
				s.Set(policy.PolicyExploreExhausted, 0)
				return nil
			}
			if s.Get(policy.StoreExploreScore) > 0 {
				s.Set(policy.CounterExploreInjected, 0)
				s.Set(policy.PolicyExploreExhausted, 0)
				return nil
			}
			if s.Flag(policy.CounterExploreInjected) {
				return nil
			}
			s.Set(policy.CounterExploreInjected, 1)
			return &policy.Result{
				Hint: &policy.Hint{Type: "exploration", Severity: "warning",
					Content: "你已经探索较多。优先基于已有信息推进实际工作；只有缺少关键事实时才继续探索。\n\n你的任务：" + s.GetStr(policy.StoreStepInput)},
				StatePatch: &policy.StatePatch{
					Ints: map[string]int64{policy.PolicyExploreExhausted: 1},
				},
			}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("explore_calls=%d has_edits=%d", s.Get(policy.StoreExploreCalls), s.Get(policy.StoreHasEdits))
		},
	}
}

func ExploreCascadeHook() policy.Hook {
	return policy.Hook{
		Name: "explore_cascade", Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			n := s.Get(policy.StoreToolResearcher)
			if n < 4 {
				return nil
			}
			return &policy.Result{Hint: &policy.Hint{Type: "explore_cascade", Severity: "warning",
				Content: fmt.Sprintf("你已经启动了 %d 个 researcher 子 Agent。如果已收集足够信息，立即综合发现并行动。\n\n你的任务：%s",
					n, s.GetStr(policy.StoreStepInput))}}
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("researcher_calls=%d", s.Get(policy.StoreToolResearcher))
		},
	}
}
