package builtin

import (
	"fmt"

	"nekocode/bot/hooks"
)

func GarbledCircuitBreaker() hooks.Hook {
	return hooks.Hook{
		Name: "garbled_circuit_breaker", Point: hooks.PostTurn,
		On: func(s hooks.State) *hooks.Result {
			if s.Get(hooks.StoreRespGarbled) >= 5 {
				stop := hooks.StopFormatError
				return &hooks.Result{Stop: &stop}
			}
			return nil
		},
		DescribeTrigger: func(s hooks.State) string {
			return fmt.Sprintf("garbled_count=%d", s.Get(hooks.StoreRespGarbled))
		},
	}
}
