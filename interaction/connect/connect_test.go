package connect

import (
	"context"
	"testing"
)

func TestBaseLifecycle(t *testing.T) {
	base := NewBase(nil, "test", "Test")
	ctx, generation := base.Start(context.Background())
	if !base.IsRunning() || generation != 1 {
		t.Fatalf("Start state = running:%v generation:%d", base.IsRunning(), generation)
	}
	if err := base.Stop(); err != nil {
		t.Fatal(err)
	}
	if base.IsRunning() {
		t.Fatal("Stop left connector running")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Stop did not cancel connector context")
	}
}
