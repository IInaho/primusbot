package taskview

import (
	"strings"
	"testing"
)

func TestMarkdownToHTMLBasicFormatting(t *testing.T) {
	got := MarkdownToHTML("Hello **bold** and `code` and ~~gone~~")
	want := "Hello <b>bold</b> and <code>code</code> and <s>gone</s>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMarkdownToHTMLEscapesPlainText(t *testing.T) {
	got := MarkdownToHTML("if a < b && c > d { }")
	if strings.Contains(got, "<b>") || !strings.Contains(got, "&lt;") || !strings.Contains(got, "&amp;&amp;") {
		t.Fatalf("plain text not escaped: %q", got)
	}
}

func TestMarkdownToHTMLFencedCodeBlock(t *testing.T) {
	src := "Before\n```go\nif a < b {\n\treturn\n}\n```\nAfter"
	got := MarkdownToHTML(src)
	if !strings.Contains(got, `<pre><code class="language-go">if a &lt; b {`) {
		t.Fatalf("fence not rendered: %q", got)
	}
	if !strings.HasSuffix(got, "After") {
		t.Fatalf("trailing text lost: %q", got)
	}
}

func TestMarkdownToHTMLUnclosedFenceDegrades(t *testing.T) {
	// Mid-stream text: the fence never closes. Must still render as <pre>
	// rather than dropping the content or emitting raw backticks.
	got := MarkdownToHTML("```\npartial code")
	if !strings.Contains(got, "<pre>partial code</pre>") {
		t.Fatalf("unclosed fence: %q", got)
	}
}

func TestMarkdownToHTMLHeadingsAndQuotes(t *testing.T) {
	got := MarkdownToHTML("## Title\n> quoted **text**")
	if !strings.Contains(got, "<b>Title</b>") {
		t.Fatalf("heading: %q", got)
	}
	if !strings.Contains(got, "<blockquote>quoted <b>text</b></blockquote>") {
		t.Fatalf("blockquote: %q", got)
	}
}

func TestMarkdownToHTMLTableMonospace(t *testing.T) {
	src := "| a | b |\n|---|---|\n| 1 | 2 |"
	got := MarkdownToHTML(src)
	if !strings.HasPrefix(got, "<pre>") || !strings.HasSuffix(got, "</pre>") {
		t.Fatalf("table should render monospace: %q", got)
	}
}

func TestMarkdownToHTMLLinks(t *testing.T) {
	got := MarkdownToHTML("see [docs](https://example.com?a=1&b=2)")
	want := `see <a href="https://example.com?a=1&amp;b=2">docs</a>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMarkdownToHTMLNoSingleStarItalics(t *testing.T) {
	// Single stars and underscores are NOT emphasis: they misfire on
	// arithmetic and snake_case identifiers.
	got := MarkdownToHTML("2 * 3 and snake_case_name")
	if strings.Contains(got, "<i>") {
		t.Fatalf("single-star/underscore must not become italics: %q", got)
	}
}

func TestMarkdownToHTMLDanglingMarkersLiteral(t *testing.T) {
	got := MarkdownToHTML("unclosed **bold and `code")
	if strings.Contains(got, "<b>") || strings.Contains(got, "<code>") {
		t.Fatalf("dangling markers must stay literal: %q", got)
	}
	if !strings.Contains(got, "**bold") {
		t.Fatalf("literal marker lost: %q", got)
	}
}

func TestMarkdownToHTMLPreservesCJK(t *testing.T) {
	// Regression: escaping must pass multi-byte UTF-8 through byte-wise;
	// string(byte) would reinterpret each byte as a code point (中 → ä¸­).
	got := MarkdownToHTML("你好,**世界**。`代码` 与 <标签>")
	want := "你好,<b>世界</b>。<code>代码</code> 与 &lt;标签&gt;"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMarkdownToHTMLLists(t *testing.T) {
	src := "- 第一项\n  - 嵌套\n* 星号项\n1. 有序保留\n- [ ] 待办\n- [x] 已办"
	got := MarkdownToHTML(src)
	for _, want := range []string{"• 第一项", "  • 嵌套", "• 星号项", "1. 有序保留", "☐ 待办", "☑ 已办"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestMarkdownToHTMLListInlineFormatting(t *testing.T) {
	got := MarkdownToHTML("- 含 **粗体** 的项")
	if !strings.Contains(got, "• 含 <b>粗体</b> 的项") {
		t.Fatalf("list inline formatting: %q", got)
	}
}

func TestMarkdownToHTMLHorizontalRule(t *testing.T) {
	got := MarkdownToHTML("上文\n---\n下文")
	if !strings.Contains(got, "───────────────") || strings.Contains(got, "---") {
		t.Fatalf("hr: %q", got)
	}
	// A list item must not be mistaken for a rule.
	got = MarkdownToHTML("- - -")
	if strings.Contains(got, "•") {
		t.Fatalf("spaced rule became a bullet: %q", got)
	}
}

func TestMarkdownToHTMLImage(t *testing.T) {
	got := MarkdownToHTML("看这里 ![架构图](https://example.com/a.png) 完")
	want := `看这里 <a href="https://example.com/a.png">🖼 架构图</a> 完`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMarkdownToHTMLBoldItalic(t *testing.T) {
	got := MarkdownToHTML("这是 ***重点*** 内容")
	if !strings.Contains(got, "<b><i>重点</i></b>") {
		t.Fatalf("bold-italic: %q", got)
	}
}
