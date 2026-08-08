package core

import "testing"

func TestParseNumstat(t *testing.T) {
	added, deleted := parseNumstat([]byte("12\t3\ta.go\x00-\t-\tasset.bin\x004\t8\tb.go\x00"))
	if added != 16 || deleted != 11 {
		t.Fatalf("parseNumstat = +%d -%d, want +16 -11", added, deleted)
	}
}

func TestCountNULTerms(t *testing.T) {
	if got := countNULTerms([]byte("a\x00b c\x00")); got != 2 {
		t.Fatalf("countNULTerms = %d, want 2", got)
	}
}
