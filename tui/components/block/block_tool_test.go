package block

import (
	"strings"
	"testing"

	"nekocode/tui/styles"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderWriteDiffSkipsHeader(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolContent(ContentBlock{
		Type:     BlockTool,
		ToolName: "write",
		Content:  "[write /tmp/file.go]\n+1:package main\n",
		Done:     true,
	}, 80, &sty)

	if strings.Contains(got, "[write /tmp/file.go]") {
		t.Fatalf("write diff header should not render:\n%s", got)
	}
	if !strings.Contains(got, "package main") {
		t.Fatalf("write diff content should render:\n%s", got)
	}
}

func TestDiffToolIsPersistent(t *testing.T) {
	if !IsPersistent("diff") {
		t.Fatal("diff should be persistent")
	}
}

func TestRenderToolsSeparatesMultipleToolBlocks(t *testing.T) {
	sty := styles.DefaultStyles()
	got := RenderTools([]ContentBlock{
		{Type: BlockTool, ToolName: "bash", ToolArgs: "go test", Done: true},
		{Type: BlockTool, ToolName: "bash", ToolArgs: "go vet", Done: true},
	}, 80, &sty)

	if !strings.Contains(got, "\n\n") {
		t.Fatalf("tool blocks should be separated by a blank line:\n%s", got)
	}
}

func TestRenderToolLineWrapsLongCommand(t *testing.T) {
	sty := styles.DefaultStyles()
	cmd := "git status --short tui/components/block/block_render.go tui/components/block/block_tool.go tui/components/block/block_tool_test.go tui/styles/colors.go tui/components/processing/processing_test.go"
	got := renderToolLine(ContentBlock{
		Type:     BlockTool,
		ToolName: "bash",
		ToolArgs: cmd,
		Done:     true,
	}, 96, &sty)
	clean := ansi.Strip(got)

	if strings.Contains(clean, "…") {
		t.Fatalf("long command should wrap instead of truncate:\n%s", got)
	}
	if !strings.Contains(clean, "Ran git status --short") {
		t.Fatalf("bash tool should render as Ran with command:\n%s", got)
	}
	if !strings.Contains(clean, "processing_test.go") {
		t.Fatalf("wrapped command should keep the tail:\n%s", got)
	}
	if !strings.Contains(clean, "\n  ") {
		t.Fatalf("long command should render continuation lines:\n%s", got)
	}
	if strings.Contains(clean, "\n   │  ") {
		t.Fatalf("continuation lines should use one space after the rail:\n%s", clean)
	}
	if !strings.Contains(clean, "\n   │ tui/components/block/block_tool.go") {
		t.Fatalf("continuation line should align with one space after rail:\n%s", clean)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(line); w > 96 {
			t.Fatalf("rendered line width = %d, want <= 96:\n%s", w, got)
		}
	}
}

func TestRenderToolLinePrefixesMultilineCommand(t *testing.T) {
	sty := styles.DefaultStyles()
	cmd := "echo one\necho two\necho three"
	got := renderToolLine(ContentBlock{
		Type:     BlockTool,
		ToolName: "bash",
		ToolArgs: cmd,
		Content:  "one\ntwo\nthree",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if !strings.Contains(clean, "Ran echo one") {
		t.Fatalf("first command line should stay in the header:\n%s", clean)
	}
	if !strings.Contains(clean, "\n   │ echo two") || !strings.Contains(clean, "\n   │ echo three") {
		t.Fatalf("multiline command continuation should use the command rail:\n%s", clean)
	}
	if strings.Contains(clean, "\necho two") || strings.Contains(clean, "\necho three") {
		t.Fatalf("multiline command should not render as raw unindented lines:\n%s", clean)
	}
	if !strings.Contains(clean, "\n    └  one") {
		t.Fatalf("tool output should remain one space deeper than command continuations:\n%s", clean)
	}
}

func TestRenderBashContentPreservesLeadingStatusSpace(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolContent(ContentBlock{
		Type:     BlockTool,
		ToolName: "bash",
		Content:  " M tui/components/block/block_tool.go\n",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if !strings.HasPrefix(clean, " M ") {
		t.Fatalf("bash output should preserve leading status space, got %q", clean)
	}
}

func TestRenderToolOutputUsesCornerConnector(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolLine(ContentBlock{
		Type:     BlockTool,
		ToolName: "bash",
		ToolArgs: `echo "hello" && date`,
		Content:  "hello\nFri Jul  3 00:41:48 CST 2026",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if !strings.Contains(clean, "\n    └  hello") {
		t.Fatalf("bash output should start with a corner connector:\n%s", clean)
	}
	if strings.Contains(clean, "\n    │  hello") || strings.Contains(clean, "\n    │  Fri") {
		t.Fatalf("bash output should not use a vertical rail:\n%s", clean)
	}

	got = renderToolLine(ContentBlock{
		Type:     BlockTool,
		ToolName: "write",
		ToolArgs: "/tmp/report.json",
		Content:  "(wrote 1234 bytes)",
		Done:     true,
	}, 80, &sty)
	clean = ansi.Strip(got)
	if !strings.Contains(clean, "\n    └  (wrote 1234 bytes)") {
		t.Fatalf("write output should use the same corner connector:\n%s", clean)
	}
	if strings.Contains(clean, "\n    │  (wrote 1234 bytes)") {
		t.Fatalf("write output should not use a vertical rail:\n%s", clean)
	}
}

func TestRenderToolLineDoesNotShowCollapseToggle(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolLine(ContentBlock{
		Type:     BlockTool,
		ToolName: "bash",
		ToolArgs: "echo hello",
		Content:  "hello",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if strings.Contains(clean, "▾") || strings.Contains(clean, "▸") {
		t.Fatalf("tool render should not show collapse toggles:\n%s", clean)
	}
}
