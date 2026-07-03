package tui

import (
	"fmt"
	"os"
	"testing"

	"nekocode/tui/components"
)

func TestNormalizeInputHistory(t *testing.T) {
	var entries []string
	for i := 0; i < maxInputHistoryEntries+5; i++ {
		entries = append(entries, fmt.Sprintf("cmd-%03d", i))
	}
	entries = append([]string{"", "  first  ", "first"}, entries...)

	got := normalizeInputHistory(entries)
	if len(got) != maxInputHistoryEntries {
		t.Fatalf("len = %d, want %d", len(got), maxInputHistoryEntries)
	}
	if got[0] != "cmd-005" {
		t.Fatalf("oldest retained entry = %q, want cmd-005", got[0])
	}
	if got[len(got)-1] != "cmd-204" {
		t.Fatalf("last entry = %q, want cmd-204", got[len(got)-1])
	}
}

func TestInputHistoryPersistsAndReloads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	history := appendInputHistory(nil, "first command")
	history = appendInputHistory(history, "second command")
	if err := saveInputHistory(history); err != nil {
		t.Fatalf("saveInputHistory: %v", err)
	}
	if _, err := os.Stat(inputHistoryPath()); err != nil {
		t.Fatalf("history file was not written: %v", err)
	}

	in := components.NewInput(80)
	in.SetHistory(loadInputHistory())
	in.HistoryUp()
	if got := in.Value(); got != "second command" {
		t.Fatalf("first history up = %q, want second command", got)
	}
	in.HistoryUp()
	if got := in.Value(); got != "first command" {
		t.Fatalf("second history up = %q, want first command", got)
	}
}
