package sandbox

import (
	"context"
	"time"

	"nekocode/bot/tools/runtime/sandbox/impl"
)

// DefaultBackend is a Backend that uses the best available OS backend
// (Linux namespaces → Landlock → tbsb). Use it when you don't need to
// inject a custom implementation.
type DefaultBackend struct{}

func (DefaultBackend) Run(ctx context.Context, command string, profile Profile, timeout time.Duration) (string, error) {
	return impl.Run(ctx, command, profile, timeout)
}

func (DefaultBackend) RunHost(ctx context.Context, command string, timeout time.Duration) (string, error) {
	return impl.RunHost(ctx, command, timeout)
}

func (DefaultBackend) IsAvailable() bool { return impl.IsAvailable() }
