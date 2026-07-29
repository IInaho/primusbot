package feishu

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestSDKLoggerImplementsInterfaceAndNeverPanics(t *testing.T) {
	l := sdkLogger{}
	ctx := context.Background()
	// Debug/Info are dropped; Warn/Error go to the debug file channel.
	l.Debug(ctx, "debug", 1)
	l.Info(ctx, "info", 2)
	l.Warn(ctx, "warn", 3)
	l.Error(ctx, "error", 4)
}

func TestSilenceStdoutAroundSwallowsAndRestores(t *testing.T) {
	orig := os.Stdout
	silenceStdoutAround(func() {
		fmt.Fprint(os.Stdout, "this line must not reach the terminal")
	})
	if os.Stdout != orig {
		t.Fatal("os.Stdout not restored")
	}
	// stdout must still be writable afterwards.
	if _, err := fmt.Fprint(os.Stdout, ""); err != nil {
		t.Fatalf("stdout broken after restore: %v", err)
	}
}
