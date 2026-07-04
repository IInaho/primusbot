package impl

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunHost_SimpleEcho(t *testing.T) {
	out, err := RunHost(t.Context(), "echo hello_host", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello_host") {
		t.Fatalf("expected 'hello_host' in output, got: %q", out)
	}
}

func TestRunHost_CommandFailure(t *testing.T) {
	_, err := RunHost(t.Context(), "exit 7", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestRunHost_Timeout(t *testing.T) {
	_, err := RunHost(t.Context(), "sleep 30", 1*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunHost_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := RunHost(ctx, "echo test", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestUnavailableError(t *testing.T) {
	e := UnavailableError{Reason: "test reason"}
	if !strings.Contains(e.Error(), "test reason") {
		t.Fatalf("unexpected error: %s", e.Error())
	}

	e2 := UnavailableError{}
	if !strings.Contains(e2.Error(), "sandbox unavailable") {
		t.Fatalf("unexpected default error: %s", e2.Error())
	}
}
