package processing

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/components/block"
	"nekocode/interaction/tui/styles"

	"github.com/charmbracelet/x/ansi"
)

func TestFinishEditRevertIsNotError(t *testing.T) {
	sty := styles.DefaultStyles()
	p := NewProcessingItem(&sty)
	p.SetBlocks([]block.ContentBlock{{
		Type:     block.BlockTool,
		ToolName: "edit",
	}})

	p.finishToolBlock("", "edit", "[/tmp/file.txt#revert]\n-1:changed\n+1:original\n", false)

	blocks := p.Blocks()
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(blocks))
	}
	if !blocks[0].Done {
		t.Fatal("revert block was not marked done")
	}
	if blocks[0].IsError {
		t.Fatal("revert block was marked as error")
	}
}

func TestBlockedPersistentToolRendersErrorContent(t *testing.T) {
	sty := styles.DefaultStyles()
	p := NewProcessingItem(&sty)
	p.SetBlocks([]block.ContentBlock{{
		Type:     block.BlockTool,
		ToolName: "edit",
		Content:  "你正在修改 x.go，但 ledger 中没有该文件的读取记录。",
		Done:     true,
		IsError:  true,
	}})

	rendered := p.renderChangesSection(100)
	if !strings.Contains(rendered, "ledger") {
		t.Fatalf("blocked edit reason not rendered:\n%s", rendered)
	}
	// Error state is now conveyed by the red accent/glyph, not a text label,
	// so we no longer assert on the literal "error" string.
}

func TestChangesSectionLeavesGapAfterToolBlocks(t *testing.T) {
	sty := styles.DefaultStyles()
	p := NewProcessingItem(&sty)
	p.SetBlocks([]block.ContentBlock{{
		Type:     block.BlockTool,
		ToolName: "write",
		Content:  "(wrote 1234 bytes)",
		Done:     true,
	}})

	rendered := p.renderChangesSection(100)
	if !strings.HasSuffix(rendered, "\n") {
		t.Fatalf("changes section should leave a trailing newline after tool blocks:\n%s", rendered)
	}
}

func TestRenderHeaderUsesSingleSpaceAfterSpinner(t *testing.T) {
	sty := styles.DefaultStyles()
	p := NewProcessingItem(&sty)
	p.SetSpinnerView("⠋")
	p.SetStatusText("Running bash")

	rendered := ansi.Strip(p.renderHeader(80))
	if !strings.Contains(rendered, "⠋ Running bash") {
		t.Fatalf("header should use one space between spinner and status: %q", rendered)
	}
	if strings.Contains(rendered, "⠋  Running bash") {
		t.Fatalf("header has extra spaces between spinner and status: %q", rendered)
	}
}
