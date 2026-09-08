package acp

import (
	"context"
	"fmt"
	"os"

	"nekocode/runtime/standard"
)

// RunStdio starts the standard NekoCode runtime as an ACP agent on stdin/stdout.
func RunStdio(ctx context.Context) error {
	return RunStdioWithOptions(ctx, ServerOptions{})
}

// RunStdioWithOptions starts the standard runtime with explicit ACP options.
func RunStdioWithOptions(ctx context.Context, options ServerOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("acp: get working directory: %w", err)
	}
	runtime, err := standard.New()
	if err != nil {
		return err
	}
	defer runtime.Close()
	return ServeWithOptions(ctx, os.Stdin, os.Stdout, runtime, cwd, options)
}
