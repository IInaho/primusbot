package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

func GarbledCircuitBreaker() policy.Hook {
	return policy.Hook{
		Name: "garbled_circuit_breaker", Point: policy.Stop,
		On: func(s policy.State) *policy.Result {
			if s.Facts().Response.GarbledCount >= 5 {
				stop := policy.StopFormatError
				return &policy.Result{Stop: &stop}
			}
			return nil
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("garbled_count=%d", s.Facts().Response.GarbledCount)
		},
	}
}
