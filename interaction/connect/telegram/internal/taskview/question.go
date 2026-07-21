package taskview

import (
	"fmt"
	"strconv"
	"strings"

	controlruntime "nekocode/runtime"
)

func UsesQuestionButtons(p controlruntime.QuestionView) bool {
	return usesQuestionButtons(p)
}

func usesQuestionButtons(p controlruntime.QuestionView) bool {
	if len(p.Questions) != 1 {
		return false
	}
	q := p.Questions[0]
	return !q.Multiple && len(q.Options) > 0
}

func questionSummary(p controlruntime.QuestionView) string {
	if len(p.Questions) == 0 {
		return compactMessage(HTMLEscape("NekoCode requested input."), labelCode("Reply", "/answer "+p.ID+" <answer>"))
	}
	if usesQuestionButtons(p) {
		q := p.Questions[0]
		header := strings.TrimSpace(q.Header)
		if header == "" {
			header = "Question"
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
			header = fmt.Sprintf("Question %d", i+1)
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
	b.WriteString(labelCode("Reply", answerCommand(p.ID)))
	b.WriteString("\n")
	b.WriteString(labelCode("Reject", dismissCommand(p.ID)))
	return b.String()
}

func answerCommand(questionID string) string {
	return "/answer " + questionID + " <answer>"
}

func dismissCommand(questionID string) string {
	return "/dismiss " + questionID
}

func buildQuestionReply(qv controlruntime.QuestionView, raw string) (controlruntime.QuestionReply, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return controlruntime.QuestionReply{}, fmt.Errorf("empty answer")
	}
	parts := splitAnswerParts(raw, len(qv.Questions))
	answers := make([][]string, len(qv.Questions))
	for i, item := range qv.Questions {
		part := ""
		if i < len(parts) {
			part = parts[i]
		}
		parsed, err := parseQuestionAnswer(item, part)
		if err != nil {
			return controlruntime.QuestionReply{}, fmt.Errorf("question %d: %w", i+1, err)
		}
		answers[i] = parsed
	}
	return controlruntime.QuestionReply{Answers: answers}, nil
}

func splitAnswerParts(raw string, count int) []string {
	if count <= 1 {
		return []string{strings.TrimSpace(raw)}
	}
	parts := strings.Split(raw, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseQuestionAnswer(item controlruntime.QuestionItem, raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty answer")
	}
	if len(item.Options) == 0 {
		return []string{raw}, nil
	}
	var tokens []string
	if item.Multiple {
		tokens = strings.Split(raw, ",")
	} else {
		tokens = []string{raw}
	}
	answers := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		label, ok := matchQuestionOption(item.Options, token)
		if !ok {
			if item.Custom {
				answers = append(answers, token)
				continue
			}
			return nil, fmt.Errorf("unknown option %q", token)
		}
		answers = append(answers, label)
	}
	if len(answers) == 0 {
		return nil, fmt.Errorf("empty answer")
	}
	return answers, nil
}

func matchQuestionOption(options []controlruntime.QuestionOption, token string) (string, bool) {
	if idx, err := strconv.Atoi(token); err == nil {
		idx--
		if idx >= 0 && idx < len(options) {
			return options[idx].Label, true
		}
	}
	folded := strings.ToLower(token)
	for _, opt := range options {
		label := strings.ToLower(opt.Label)
		if folded == label || strings.HasPrefix(label, folded) {
			return opt.Label, true
		}
	}
	return "", false
}
