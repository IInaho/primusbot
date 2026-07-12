package message

import (
	"strings"
	"testing"

	"nekocode/tui/components/block"
	"nekocode/tui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestAssistantMessagePreservesToolBlockIndentation(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewAssistantMessageItem(&sty, "This is a long assistant sentence that should wrap without inheriting tool indentation in the final message body.")
	m.SetBlocks([]block.ContentBlock{{
		Type:     block.BlockTool,
		ToolName: "shell",
		ToolArgs: `echo "hello" && date`,
		Content:  "hello\nFri Jul  3 00:41:48 CST 2026",
		Done:     true,
	}})

	clean := ansi.Strip(m.Render(100))
	if strings.Contains(clean, "Assistant") || strings.Contains(clean, "▐") {
		t.Fatalf("assistant message should not render a label or left rail:\n%s", clean)
	}
	if !strings.Contains(clean, "────────────") {
		t.Fatalf("assistant message should start with a separator:\n%s", clean)
	}
	if !strings.Contains(clean, "\n  ฅ Ran") {
		t.Fatalf("assistant tool header should preserve inner indentation:\n%s", clean)
	}
	if !strings.Contains(clean, "\n    └  hello") {
		t.Fatalf("assistant shell output connector should stay under Ran:\n%s", clean)
	}
	if strings.Contains(clean, "\n└  hello") || strings.Contains(clean, "\n│") {
		t.Fatalf("assistant render stripped tool indentation:\n%s", clean)
	}
	if !strings.Contains(clean, "\n  • This is") {
		t.Fatalf("assistant body should use a bullet prefix for the reply text:\n%s", clean)
	}
}

func TestAssistantMessageBodyUsesSeparator(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewAssistantMessageItem(&sty, "First paragraph with enough text to wrap onto another line at a narrow width.\n\nSecond paragraph.")

	clean := ansi.Strip(m.Render(58))
	if strings.Contains(clean, "Assistant") || strings.Contains(clean, "▐") {
		t.Fatalf("assistant body should not render a label or left rail:\n%s", clean)
	}
	if !strings.HasPrefix(clean, "────────────") {
		t.Fatalf("assistant body should start with a separator:\n%s", clean)
	}
	firstLine := strings.Split(clean, "\n")[0]
	if w := lipgloss.Width(firstLine); w != 58 {
		t.Fatalf("assistant separator should span message width, width=%d:\n%s", w, clean)
	}
	if !strings.Contains(clean, "\n  • First paragraph") {
		t.Fatalf("assistant body should render text below separator:\n%s", clean)
	}
	if strings.Contains(clean, "\n  • Second paragraph.") {
		t.Fatalf("assistant body should only render one bullet per reply:\n%s", clean)
	}
	if !strings.Contains(clean, "\n    Second paragraph.") {
		t.Fatalf("later paragraphs should align with reply text:\n%s", clean)
	}
}

func TestAssistantMessageFooterIsIndented(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewAssistantMessageItem(&sty, "Done")
	m.SetFooter("Duration: 16.3s  ↑2.4k ↓126")

	clean := ansi.Strip(m.Render(80))
	if !strings.Contains(clean, "\n  Duration: 16.3s  ↑2.4k ↓126") {
		t.Fatalf("assistant footer should be indented:\n%s", clean)
	}
	if strings.Contains(clean, "\nDuration: 16.3s") {
		t.Fatalf("assistant footer should not start at column zero:\n%s", clean)
	}
}
