package app

import (
	"testing"

	"nekocode/bot/view"
)

func TestCommandResult(t *testing.T) {
	if got := commandResult(true, true); got != view.CmdConfirming {
		t.Fatalf("pending confirm wins, got %v", got)
	}
	if got := commandResult(false, true); got != view.CmdSessionResumed {
		t.Fatalf("session resumed = %v", got)
	}
	if got := commandResult(false, false); got != view.CmdHandled {
		t.Fatalf("handled = %v", got)
	}
}
