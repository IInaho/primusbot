package components

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/styles"
	controlruntime "nekocode/runtime"
)

func TestSuggestionsSeparateSlashAndDollarCommands(t *testing.T) {
	sty := styles.DefaultStyles()
	s := NewSuggestions(&sty)
	commands := menuItems("/help", "/plugin", "$review", "$write")

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

	s.Refresh("/h", menuItems("help"))

	if got, ok := s.Accept(); !ok || got.Value != "/help" {
		t.Fatalf("Accept() = %+v, %v, want /help", got, ok)
	}
}

func menuItems(values ...string) []controlruntime.CommandMenuItem {
	items := make([]controlruntime.CommandMenuItem, 0, len(values))
	for _, value := range values {
		items = append(items, controlruntime.CommandMenuItem{Value: value, Label: value})
	}
	return items
}

func TestSuggestionsRenderNestedMenuWithAlignedThemeRail(t *testing.T) {
	sty := styles.DefaultStyles()
	s := NewSuggestions(&sty)
	s.OpenMenu("Choose model", "", []controlruntime.CommandMenuItem{
		{Value: "/model fast", Label: "fast", Description: "openai / gpt-fast · current", Submit: true},
		{Value: "/model deep", Label: "deep-reasoner", Description: "anthropic / claude"},
	})

	view := s.View(72)
	for _, want := range []string{"Choose model · 2", styles.HeavyVert, "▸ fast", "gpt-fast", "↑↓ move", "esc close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("menu missing %q:\n%s", want, view)
		}
	}
	if s.Height() != 5 {
		t.Fatalf("menu height = %d, want 5", s.Height())
	}
	if !strings.Contains(view, "\n\n") {
		t.Fatalf("menu lacks spacing before key hints:\n%s", view)
	}
}

func TestSuggestionsCycleWrapsNestedMenu(t *testing.T) {
	sty := styles.DefaultStyles()
	s := NewSuggestions(&sty)
	s.OpenMenu("Models", "", []controlruntime.CommandMenuItem{{Label: "one"}, {Label: "two"}})
	s.Cycle(-1)
	got, ok := s.Accept()
	if !ok || got.Label != "two" {
		t.Fatalf("wrapped selection = %+v, %v", got, ok)
	}
}
