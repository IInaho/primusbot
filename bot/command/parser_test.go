package command

import (
	"testing"
)

func TestParserParse(t *testing.T) {
	p := NewParser()

	tests := []struct {
		input      string
		wantPrefix string
		wantName   string
		wantArgs   int
	}{
		{"/help", "/", "help", 0},
		{"/plan do something", "/", "plan", 2},
		{"not a command", "", "", 0},
		{"/STATS", "/", "stats", 0},
		{"$review fix this", "$", "review", 2},
	}
	for _, tt := range tests {
		cmd := p.Parse(tt.input)
		if cmd.Prefix != tt.wantPrefix {
			t.Errorf("Parse(%q).Prefix = %q, want %q", tt.input, cmd.Prefix, tt.wantPrefix)
		}
		if cmd.Name != tt.wantName {
			t.Errorf("Parse(%q).Name = %q, want %q", tt.input, cmd.Name, tt.wantName)
		}
		if len(cmd.Args) != tt.wantArgs {
			t.Errorf("Parse(%q).Args len = %d, want %d", tt.input, len(cmd.Args), tt.wantArgs)
		}
	}
}

func TestParserExecute(t *testing.T) {
	p := NewParser()
	p.Register("test", func(cmd *Command) (string, bool) { return "ok", true })
	p.RegisterDynamic("review", func(cmd *Command) (string, bool) { return "dynamic", true })

	// Unknown command.
	msg, handled := p.Execute(&Command{Name: "unknown"})
	if !handled || msg != "Unknown command: /unknown. Type /help for available commands." {
		t.Errorf("unexpected: %q, %v", msg, handled)
	}

	// Known command.
	msg, handled = p.Execute(&Command{Name: "test"})
	if !handled || msg != "ok" {
		t.Errorf("unexpected: %q, %v", msg, handled)
	}

	// Dynamic command.
	msg, handled = p.Execute(&Command{Prefix: "$", Name: "review"})
	if !handled || msg != "dynamic" {
		t.Errorf("unexpected: %q, %v", msg, handled)
	}

	// Empty command.
	_, handled = p.Execute(&Command{Name: ""})
	if handled {
		t.Error("empty command should not be handled")
	}
}

func TestParserCommands(t *testing.T) {
	p := NewParser()
	p.Register("a", nil)
	p.RegisterDynamic("b", nil)
	names := p.Commands()
	want := []string{"$b", "/a"}
	if len(names) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(names), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("Commands()[%d] = %q, want %q (all: %v)", i, names[i], want[i], names)
		}
	}
}
