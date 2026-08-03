package taskview

import (
	"fmt"
	"strings"

	"nekocode/interaction/connect"
	controlruntime "nekocode/runtime"
)

// questionSummary renders the question body. Option questions that fit the
// inline keyboard get a compact rendering (buttons carry the answer path);
// free-form and multi-part questions carry the /answer //dismiss commands.
func questionSummary(p controlruntime.QuestionView) string {
	if len(p.Questions) == 0 {
		return compactMessage(HTMLEscape("NekoCode 请求输入。"), labelCode("回复", "/answer "+p.ID+" <answer>"))
	}
	if connect.QuestionSelectable(p) {
		q := p.Questions[0]
		header := strings.TrimSpace(q.Header)
		if header == "" {
			header = "提问"
		}
		return compactMessage(htmlTitle(header), HTMLEscape(q.Question))
	}
	var b strings.Builder
	for i, q := range p.Questions {
		if i > 0 {
			b.WriteString("\n\n")
		}
		header := strings.TrimSpace(q.Header)
		if header == "" {
			header = fmt.Sprintf("提问 %d", i+1)
		}
		fmt.Fprintf(&b, "%s\n%s", htmlTitle(header), HTMLEscape(q.Question))
		if len(q.Options) > 0 {
			b.WriteString("\n")
			for idx, opt := range q.Options {
				fmt.Fprintf(&b, "\n%d. %s", idx+1, HTMLEscape(opt.Label))
				if opt.Description != "" {
					b.WriteString(": ")
					b.WriteString(HTMLEscape(opt.Description))
				}
			}
		}
	}
	b.WriteString("\n\n")
	b.WriteString(labelCode("回复", "/answer "+p.ID+" <answer>"))
	b.WriteString("\n")
	b.WriteString(labelCode("忽略", "/dismiss "+p.ID))
	return b.String()
}
