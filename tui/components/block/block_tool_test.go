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
		{Type: BlockTool, ToolName: "shell", ToolArgs: "go test", Done: true},
		{Type: BlockTool, ToolName: "shell", ToolArgs: "go vet", Done: true},
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
		ToolName: "shell",
		ToolArgs: cmd,
		Done:     true,
	}, 96, &sty)
	clean := ansi.Strip(got)

	if strings.Contains(clean, "…") {
		t.Fatalf("long command should wrap instead of truncate:\n%s", got)
	}
	if !strings.Contains(clean, "Ran git status --short") {
		t.Fatalf("shell tool should render as Ran with command:\n%s", got)
	}
	if !strings.Contains(clean, "processing_test.go") {
		t.Fatalf("wrapped command should keep the tail:\n%s", got)
	}
	if !strings.Contains(clean, "\n  ") {
		t.Fatalf("long command should render continuation lines:\n%s", got)
	}
	if strings.Contains(clean, "\n    │  ") {
		t.Fatalf("continuation lines should use one space after the rail:\n%s", clean)
	}
	if !strings.Contains(clean, "\n    │ tui/components/block/block_tool.go") {
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
		ToolName: "shell",
		ToolArgs: cmd,
		Content:  "one\ntwo\nthree",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if !strings.Contains(clean, "Ran echo one") {
		t.Fatalf("first command line should stay in the header:\n%s", clean)
	}
	if !strings.Contains(clean, "\n    │ echo two") || !strings.Contains(clean, "\n    │ echo three") {
		t.Fatalf("multiline command continuation should use the command rail:\n%s", clean)
	}
	if strings.Contains(clean, "\necho two") || strings.Contains(clean, "\necho three") {
		t.Fatalf("multiline command should not render as raw unindented lines:\n%s", clean)
	}
	if !strings.Contains(clean, "\n    └  one") {
		t.Fatalf("tool output should remain one space deeper than command continuations:\n%s", clean)
	}
}

func TestRenderShellContentPreservesLeadingStatusSpace(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolContent(ContentBlock{
		Type:     BlockTool,
		ToolName: "shell",
		Content:  " M tui/components/block/block_tool.go\n",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if !strings.HasPrefix(clean, " M ") {
		t.Fatalf("shell output should preserve leading status space, got %q", clean)
	}
}

func TestRenderShellContentKeepsHeadAndTail(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolContent(ContentBlock{
		Type:     BlockTool,
		ToolName: "shell",
		Content:  "line 0\nline 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\n",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	for _, want := range []string{"line 0", "line 1", "line 2", "line 6", "line 7", "line 8"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("missing retained line %q:\n%s", want, clean)
		}
	}
	for _, notWant := range []string{"line 3", "line 5"} {
		if strings.Contains(clean, notWant) {
			t.Fatalf("middle line %q should be hidden:\n%s", notWant, clean)
		}
	}
	if !strings.Contains(clean, "3 lines truncated") {
		t.Fatalf("missing truncation marker:\n%s", clean)
	}
}

func TestRenderToolOutputUsesCornerConnector(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolLine(ContentBlock{
		Type:     BlockTool,
		ToolName: "shell",
		ToolArgs: `echo "hello" && date`,
		Content:  "hello\nFri Jul  3 00:41:48 CST 2026",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if !strings.Contains(clean, "\n    └  hello") {
		t.Fatalf("shell output should start with a corner connector:\n%s", clean)
	}
	if strings.Contains(clean, "\n    │  hello") || strings.Contains(clean, "\n    │  Fri") {
		t.Fatalf("shell output should not use a vertical rail:\n%s", clean)
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
		ToolName: "shell",
		ToolArgs: "echo hello",
		Content:  "hello",
		Done:     true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if strings.Contains(clean, "▾") || strings.Contains(clean, "▸") {
		t.Fatalf("tool render should not show collapse toggles:\n%s", clean)
	}
}

func TestRenderShellListUsesActionWording(t *testing.T) {
	sty := styles.DefaultStyles()
	got := renderToolLine(ContentBlock{
		Type:       BlockTool,
		ToolName:   "shell",
		ToolAction: "list",
		ToolArgs:   "shell sessions",
		Content:    "(no shell sessions)",
		Done:       true,
	}, 80, &sty)
	clean := ansi.Strip(got)

	if strings.Contains(clean, "Ran") {
		t.Fatalf("shell list should not render as Ran:\n%s", clean)
	}
	if !strings.Contains(clean, "Listed shell sessions") {
		t.Fatalf("shell list should describe the action:\n%s", clean)
	}
	if !strings.Contains(clean, "No active shell sessions") {
		t.Fatalf("empty shell list should be user-facing:\n%s", clean)
	}
}
