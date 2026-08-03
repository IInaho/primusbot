package taskview

import (
	"strings"
)

// MarkdownToHTML converts the common LLM markdown subset into Telegram-safe
// HTML (parse_mode=HTML). The subset is deliberately conservative — fenced
// code blocks, inline code, bold, strikethrough, links, headings,
// blockquotes, and pipe tables (rendered monospace) — because Telegram is
// not a markdown renderer: headings, lists and tables have no native
// representation, and single-star/underscore italics break constantly on
// code identifiers (snake_case) and math. Anything unrecognized is escaped
// and shown literally. Unclosed constructs (mid-stream fences, dangling
// bold) degrade gracefully, so partial streaming text renders sanely; any
// residual parse failure is covered by the sender's plain-text retry.
func MarkdownToHTML(src string) string {
	lines := strings.Split(src, "\n")
	var out []string

	var para []string
	flushPara := func() {
		if len(para) > 0 {
			out = append(out, inlineMarkdown(strings.Join(para, "\n")))
			para = nil
		}
	}

	inFence := false
	fenceLang := ""
	var fenceBody []string
	emitFence := func() {
		body := HTMLEscape(strings.Join(fenceBody, "\n"))
		if fenceLang != "" {
			out = append(out, `<pre><code class="language-`+fenceLang+`">`+body+"</code></pre>")
		} else {
			out = append(out, "<pre>"+body+"</pre>")
		}
		inFence = false
		fenceLang = ""
		fenceBody = nil
	}

	var table []string
	flushTable := func() {
		if len(table) > 0 {
			out = append(out, "<pre>"+HTMLEscape(strings.Join(table, "\n"))+"</pre>")
			table = nil
		}
	}

	var quote []string
	flushQuote := func() {
		if len(quote) > 0 {
			out = append(out, "<blockquote>"+inlineMarkdown(strings.Join(quote, "\n"))+"</blockquote>")
			quote = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				emitFence()
			} else {
				flushPara()
				flushTable()
				flushQuote()
				inFence = true
				fenceLang = sanitizeLang(strings.TrimSpace(trimmed[3:]))
				fenceBody = nil
			}
			continue
		}
		if inFence {
			fenceBody = append(fenceBody, line)
			continue
		}
		if strings.HasPrefix(trimmed, "|") {
			flushPara()
			flushQuote()
			table = append(table, line)
			continue
		}
		flushTable()
		if text, ok := strings.CutPrefix(trimmed, "> "); ok {
			flushPara()
			quote = append(quote, text)
			continue
		}
		flushQuote()
		if heading, ok := headingText(trimmed); ok {
			flushPara()
			out = append(out, "<b>"+inlineMarkdown(heading)+"</b>")
			continue
		}
		if isHorizontalRule(trimmed) {
			flushPara()
			out = append(out, "───────────────")
			continue
		}
		para = append(para, renderListLine(line))
	}
	if inFence {
		emitFence() // unclosed fence (mid-stream): render what we have
	}
	flushPara()
	flushTable()
	flushQuote()
	return strings.Join(out, "\n")
}

// isHorizontalRule reports whether the line is a markdown horizontal rule
// (---, ***, ___, optionally spaced).
func isHorizontalRule(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	var marker byte
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c == ' ' {
			continue
		}
		if c != '-' && c != '*' && c != '_' {
			return false
		}
		if marker == 0 {
			marker = c
		} else if c != marker {
			return false
		}
	}
	return marker != 0
}

// renderListLine rewrites markdown list markers into plain-text bullets
// that read well in Telegram (which has no native list markup): unordered
// markers (-, *, +) become "•", task checkboxes become ☐/☑, indentation is
// preserved for nesting. Ordered lists (1. ...) already read fine as-is.
func renderListLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	rest, ok := strings.CutPrefix(trimmed, "- ")
	if !ok {
		rest, ok = strings.CutPrefix(trimmed, "* ")
	}
	if !ok {
		rest, ok = strings.CutPrefix(trimmed, "+ ")
	}
	if !ok {
		return line
	}
	if task, ok := strings.CutPrefix(rest, "[ ] "); ok {
		return indent + "☐ " + task
	}
	if task, ok := strings.CutPrefix(rest, "[x] "); ok {
		return indent + "☑ " + task
	}
	if task, ok := strings.CutPrefix(rest, "[X] "); ok {
		return indent + "☑ " + task
	}
	return indent + "• " + rest
}

// headingText strips an ATX heading marker (# .. ######) and returns the
// heading text.
func headingText(trimmed string) (string, bool) {
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i == 0 || i > 6 || i >= len(trimmed) || trimmed[i] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[i+1:]), true
}

// sanitizeLang keeps a fence info string safe for embedding in a class
// attribute (letters, digits, dash, plus, hash).
func sanitizeLang(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '+' || r == '#' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// inlineMarkdown handles inline constructs: `code`, **bold**, ***bold
// italic***, ~~strike~~, [text](url), and ![alt](url) images (linkified —
// a text message cannot embed external images). Everything else is
// HTML-escaped literally. Deliberately no single-star or underscore
// italics — they misfire on code identifiers and arithmetic far more
// often than they render real emphasis.
func inlineMarkdown(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '`' {
			if end := strings.IndexByte(s[i+1:], '`'); end > 0 {
				b.WriteString("<code>" + HTMLEscape(s[i+1:i+1+end]) + "</code>")
				i += end + 2
				continue
			}
		}
		if strings.HasPrefix(s[i:], "***") {
			if end := strings.Index(s[i+3:], "***"); end > 0 {
				b.WriteString("<b><i>" + inlineMarkdown(s[i+3:i+3+end]) + "</i></b>")
				i += end + 6
				continue
			}
		}
		if strings.HasPrefix(s[i:], "**") {
			if end := strings.Index(s[i+2:], "**"); end > 0 {
				b.WriteString("<b>" + inlineMarkdown(s[i+2:i+2+end]) + "</b>")
				i += end + 4
				continue
			}
		}
		if strings.HasPrefix(s[i:], "~~") {
			if end := strings.Index(s[i+2:], "~~"); end > 0 {
				b.WriteString("<s>" + inlineMarkdown(s[i+2:i+2+end]) + "</s>")
				i += end + 4
				continue
			}
		}
		if strings.HasPrefix(s[i:], "![") {
			if mid := strings.Index(s[i:], "]("); mid > 0 {
				if close := strings.IndexByte(s[i+mid+2:], ')'); close >= 0 {
					alt := s[i+2 : i+mid]
					url := s[i+mid+2 : i+mid+2+close]
					if alt == "" {
						alt = "图片"
					}
					b.WriteString(`<a href="` + HTMLEscape(url) + `">🖼 ` + inlineMarkdown(alt) + `</a>`)
					i += mid + 2 + close + 1
					continue
				}
			}
		}
		if s[i] == '[' {
			if mid := strings.Index(s[i:], "]("); mid > 0 {
				if close := strings.IndexByte(s[i+mid+2:], ')'); close >= 0 {
					text := s[i+1 : i+mid]
					url := s[i+mid+2 : i+mid+2+close]
					b.WriteString(`<a href="` + HTMLEscape(url) + `">` + inlineMarkdown(text) + `</a>`)
					i += mid + 2 + close + 1
					continue
				}
			}
		}
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			// Write the raw byte, NOT string(byte) — the latter would
			// reinterpret the byte as a code point and mangle every
			// multi-byte UTF-8 sequence (CJK text becomes mojibake).
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String()
}
