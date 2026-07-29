package llmstream

import (
	"context"

	"nekocode/bot/provider"
	"nekocode/common/debug"
)

// CallLLMWithRetry calls CallLLM, rebuilding the call options on every
// retryable failure via buildOptions.
func CallLLMWithRetry(ctx context.Context, client provider.LLM, buildOptions func() LLMCallOptions) (*LLMCallResult, error) {
	var result *LLMCallResult
	err := WithRetry(ctx, func() error {
		var err error
		result, err = CallLLM(client, buildOptions())
		return err
	})
	return result, err
}

// WithRetry runs fn under the provider-default retry policy, logging each
// retryable attempt.
func WithRetry(ctx context.Context, fn func() error) error {
	var attempt int
	return provider.Retry(ctx, provider.DefaultRetryConfig, func() error {
		err := fn()
		if err != nil && provider.IsRetryable(err) {
			attempt++
			debug.Log("retry %d: %v", attempt, err)
		}
		return err
	})
}
