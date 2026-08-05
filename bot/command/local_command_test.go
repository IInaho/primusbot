package command

import (
	"context"
	"strings"
	"testing"
)

func TestCommandAvailabilityClassifiesLocalCommands(t *testing.T) {
	p := NewParser()
	RegisterDefaults(p, Deps{})

	for _, input := range []string{"/help", "/context", "/config", "/permission", "/permission full"} {
		isCmd, during := p.CommandAvailability(input)
		if !isCmd || !during {
			t.Errorf("%q: isCommand=%v duringTask=%v, want local command", input, isCmd, during)
		}
	}
	for _, input := range []string{"/new", "/clear", "/summarize", "/model", "/rewind"} {
		isCmd, during := p.CommandAvailability(input)
		if !isCmd || during {
			t.Errorf("%q: isCommand=%v duringTask=%v, want run-path command", input, isCmd, during)
		}
	}
	for _, input := range []string{"hello", "/nonexistent", "", "/"} {
		isCmd, _ := p.CommandAvailability(input)
		if isCmd {
			t.Errorf("%q: unexpectedly classified as command", input)
		}
	}
}

func TestLocalCommandExecutesWithoutRunPath(t *testing.T) {
	p := NewParser()
	var full bool
	RegisterDefaults(p, Deps{
		SetFullAccess: func(on bool) { full = on },
		GetFullAccess: func() bool { return full },
	})

	isCmd, during := p.CommandAvailability("/permission full")
	if !isCmd || !during {
		t.Fatal("/permission must be a local command")
	}
	out, handled := p.Execute(context.Background(), p.Parse("/permission full"))
	if !handled || !full || !strings.Contains(out, "全接管") {
		t.Fatalf("local execution failed: handled=%v full=%v out=%q", handled, full, out)
	}
}
