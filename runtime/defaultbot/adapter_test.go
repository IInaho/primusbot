package defaultbot

import (
	"testing"
	"time"

	commonview "nekocode/common/view"
	controlruntime "nekocode/runtime"
)

type mockBot struct {
	configuredCallbacks commonview.ControlCallbacks
	runInput            string
	runCallbacks        commonview.RunCallbacks
	runResult           string
	runErr              error
}

func (m *mockBot) Run(input string, callbacks commonview.RunCallbacks) (string, error) {
	m.runInput = input
	m.runCallbacks = callbacks
	return m.runResult, m.runErr
}
func (m *mockBot) ConfigureRuntime(callbacks commonview.ControlCallbacks) {
	m.configuredCallbacks = callbacks
}
func (m *mockBot) Steer(string) {}
func (m *mockBot) Abort()       {}
func (m *mockBot) Close()       {}

func TestAdapterForwardsRunCallbacks(t *testing.T) {
	bot := &mockBot{runResult: "result"}
	a := &adapter{bot: bot}

	var deltas []string
	result, err := a.Run("hello", controlruntime.RunCallbacks{
		Text: func(delta string) { deltas = append(deltas, delta) },
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result != "result" {
		t.Fatalf("Run result = %q, want %q", result, "result")
	}
	if bot.runInput != "hello" {
		t.Fatalf("bot input = %q, want %q", bot.runInput, "hello")
	}

	// Exercise the callback bridge.
	bot.runCallbacks.Text("delta")
	if len(deltas) != 1 || deltas[0] != "delta" {
		t.Fatalf("text deltas = %v, want [delta]", deltas)
	}
}

func TestAdapterConfiguresConfirmCh(t *testing.T) {
	bot := &mockBot{}
	a := &adapter{bot: bot}

	coreCh := make(chan controlruntime.ConfirmRequest, 1)
	a.ConfigureRuntime(controlruntime.ControlCallbacks{
		ConfirmCh: coreCh,
	})

	if bot.configuredCallbacks.ConfirmCh == nil {
		t.Fatal("ConfirmCh was not forwarded")
	}

	// Send an unblock signal from the bot side and verify it reaches the runtime.
	bot.configuredCallbacks.ConfirmCh <- commonview.ConfirmRequest{}
	var req controlruntime.ConfirmRequest
	select {
	case req = <-coreCh:
	case <-time.After(time.Second):
		t.Fatal("unblock signal was not forwarded to runtime ConfirmCh")
	}
	if req.Response != nil {
		t.Fatalf("expected nil Response for unblock signal, got %v", req.Response)
	}
}
