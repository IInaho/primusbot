package components

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/styles"
)

func TestSuggestionsSeparateSlashAndDollarCommands(t *testing.T) {
	sty := styles.DefaultStyles()
	s := NewSuggestions(&sty)
	commands := []string{"/help", "/plugin", "$review", "$write"}

	s.Refresh("/", commands)
	view := s.View(80)
	if !strings.Contains(view, "/help") || !strings.Contains(view, "/plugin") {
		t.Fatalf("slash suggestions missing slash commands:\n%s", view)
	}
	if strings.Contains(view, "$review") || strings.Contains(view, "$write") {
		t.Fatalf("slash suggestions should not include dynamic commands:\n%s", view)
	}

	s.Refresh("$", commands)
	view = s.View(80)
	if !strings.Contains(view, "$review") || !strings.Contains(view, "$write") {
		t.Fatalf("dollar suggestions missing dynamic commands:\n%s", view)
	}
	if strings.Contains(view, "/help") || strings.Contains(view, "/plugin") {
		t.Fatalf("dollar suggestions should not include slash commands:\n%s", view)
	}
}

func TestSuggestionsAcceptBareCommandNamesAsSlashCommands(t *testing.T) {
	sty := styles.DefaultStyles()
	s := NewSuggestions(&sty)

	s.Refresh("/h", []string{"help"})

	if got := s.Accept(); got != "/help" {
		t.Fatalf("Accept() = %q, want /help", got)
	}
}
