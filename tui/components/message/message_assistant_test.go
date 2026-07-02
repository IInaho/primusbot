package message

import (
	"strings"
	"testing"

	"nekocode/tui/components/block"
	"nekocode/tui/styles"

	"github.com/charmbracelet/x/ansi"
)

func TestAssistantMessagePreservesToolBlockIndentation(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewAssistantMessageItem(&sty, "This is a long assistant sentence that should wrap without inheriting tool indentation in the final message body.")
	m.SetBlocks([]block.ContentBlock{{
		Type:     block.BlockTool,
		ToolName: "bash",
		ToolArgs: `echo "hello" && date`,
		Content:  "hello\nFri Jul  3 00:41:48 CST 2026",
		Done:     true,
	}})

	clean := ansi.Strip(m.Render(100))
	if !strings.Contains(clean, "▐   ฅ Ran") {
		t.Fatalf("assistant tool header should preserve inner indentation:\n%s", clean)
	}
	if !strings.Contains(clean, "\n▐     └  hello") {
		t.Fatalf("assistant bash output connector should stay under Ran:\n%s", clean)
	}
	if strings.Contains(clean, "\n▐ └  hello") || strings.Contains(clean, "\n▐ │") {
		t.Fatalf("assistant render stripped tool indentation:\n%s", clean)
	}
	if strings.Contains(clean, "\n▐   This is") {
		t.Fatalf("assistant body should not inherit tool indentation:\n%s", clean)
	}
}
