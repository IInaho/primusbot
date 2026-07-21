package builtin

import (
	"fmt"

	"nekocode/bot/hooks"
)

func ExplorationExhaustedHook() hooks.Hook {
	return hooks.Hook{
		Name: "exploration_exhausted", Point: hooks.PreTurn,
		On: func(s hooks.State) *hooks.Result {
			if s.Get(hooks.StoreExploreCalls) < 10 {
				s.Set(hooks.CounterExploreInjected, 0)
				s.Set(hooks.PolicyExploreExhausted, 0)
				return nil
			}
			if s.Get(hooks.StoreExploreScore) > 0 {
				s.Set(hooks.CounterExploreInjected, 0)
				s.Set(hooks.PolicyExploreExhausted, 0)
				return nil
			}
			if s.Flag(hooks.CounterExploreInjected) {
				return nil
			}
			s.Set(hooks.CounterExploreInjected, 1)
			return &hooks.Result{
				Hint: &hooks.Hint{Type: "exploration", Severity: "warning",
					Content: "你已经探索较多。优先基于已有信息推进实际工作；只有缺少关键事实时才继续探索。\n\n你的任务：" + s.GetStr(hooks.StoreStepInput)},
				StatePatch: &hooks.StatePatch{
					Ints: map[string]int64{hooks.PolicyExploreExhausted: 1},
				},
			}
		},
		DescribeTrigger: func(s hooks.State) string {
			return fmt.Sprintf("explore_calls=%d has_edits=%d", s.Get(hooks.StoreExploreCalls), s.Get(hooks.StoreHasEdits))
		},
	}
}

func ExploreCascadeHook() hooks.Hook {
	return hooks.Hook{
		Name: "explore_cascade", Point: hooks.PostTool,
		On: func(s hooks.State) *hooks.Result {
			n := s.Get(hooks.StoreToolResearcher)
			if n < 4 {
				return nil
			}
			return &hooks.Result{Hint: &hooks.Hint{Type: "explore_cascade", Severity: "warning",
				Content: fmt.Sprintf("你已经启动了 %d 个 researcher 子 Agent。如果已收集足够信息，立即综合发现并行动。\n\n你的任务：%s",
					n, s.GetStr(hooks.StoreStepInput))}}
		},
		DescribeTrigger: func(s hooks.State) string {
			return fmt.Sprintf("researcher_calls=%d", s.Get(hooks.StoreToolResearcher))
		},
	}
}
