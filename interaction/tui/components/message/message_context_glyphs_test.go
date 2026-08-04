package message

import (
	"strings"
	"testing"
)

func TestColorizeContextGlyphsColorsBar(t *testing.T) {
	in := "⛁ ⛁ ⛀ ⛶ ⛶  192.3k / 1.0m"
	out := colorizeContextGlyphs(in)
	if out == in {
		t.Fatal("glyphs were not colorized")
	}
	for _, g := range []string{"⛁", "⛀", "⛶"} {
		if !strings.Contains(out, g) {
			t.Fatalf("glyph %s lost in output: %q", g, out)
		}
	}
	if !strings.Contains(out, "192.3k / 1.0m") {
		t.Fatalf("text content altered: %q", out)
	}
}

func TestColorizeContextGlyphsPassthrough(t *testing.T) {
	in := "plain system message 普通文本"
	if got := colorizeContextGlyphs(in); got != in {
		t.Fatalf("plain text changed: %q", got)
	}
}
