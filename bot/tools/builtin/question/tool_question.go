package question

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"nekocode/bot/tools/runtime/core"
	"nekocode/protocol"
)

type Tool struct {
	mu sync.RWMutex
	fn protocol.QuestionFunc
}

func NewTool() *Tool { return &Tool{} }

func (t *Tool) Name() string                                    { return "question" }
func (t *Tool) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeSequential }

func (t *Tool) Description() string {
	return "Ask the user structured questions during execution and wait for their answers. Use this when a decision, preference, or clarification is required before continuing."
}

func (t *Tool) Parameters() []core.Parameter {
	return []core.Parameter{
		{
			Name:        "questions",
			Type:        "array",
			Required:    true,
			Description: "Questions to ask. Each item has question, optional header, options [{label, description}], optional multiple, and optional custom.",
		},
	}
}

func (t *Tool) SetQuestionFunc(fn protocol.QuestionFunc) {
	t.mu.Lock()
	t.fn = fn
	t.mu.Unlock()
}

func (t *Tool) questionFunc() protocol.QuestionFunc {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.fn
}

func (t *Tool) Execute(ctx context.Context, args map[string]any) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	questions, err := parseQuestions(args)
	if err != nil {
		return "", err
	}
	fn := t.questionFunc()
	if fn == nil {
		return "", fmt.Errorf("question UI is not available")
	}

	req := protocol.NewQuestionRequest(questions)
	reply := fn(req)
	if reply.Rejected {
		return "", fmt.Errorf("question rejected")
	}
	return formatAnswers(questions, reply.Answers), nil
}

func parseQuestions(args map[string]any) ([]protocol.QuestionItem, error) {
	raw, ok := args["questions"].([]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("questions must be a non-empty array")
	}
	out := make([]protocol.QuestionItem, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("questions[%d] must be an object", i)
		}
		q, _ := m["question"].(string)
		q = strings.TrimSpace(q)
		if q == "" {
			return nil, fmt.Errorf("questions[%d].question is required", i)
		}
		header, _ := m["header"].(string)
		out = append(out, protocol.QuestionItem{
			Header:   strings.TrimSpace(header),
			Question: q,
			Options:  parseOptions(m["options"]),
			Multiple: boolArg(m["multiple"]),
			Custom:   customArg(m["custom"]),
		})
	}
	return out, nil
}

func parseOptions(raw any) []protocol.QuestionOption {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	options := make([]protocol.QuestionOption, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		label, _ := m["label"].(string)
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		desc, _ := m["description"].(string)
		options = append(options, protocol.QuestionOption{
			Label:       label,
			Description: strings.TrimSpace(desc),
		})
	}
	return options
}

func boolArg(v any) bool {
	b, _ := v.(bool)
	return b
}

func customArg(v any) bool {
	if v == nil {
		return true
	}
	b, ok := v.(bool)
	return ok && b
}

func formatAnswers(questions []protocol.QuestionItem, answers [][]string) string {
	var b strings.Builder
	b.WriteString("User answered the questions:\n")
	for i, q := range questions {
		answer := "Unanswered"
		if i < len(answers) && len(answers[i]) > 0 {
			answer = strings.Join(answers[i], ", ")
		}
		fmt.Fprintf(&b, "- %s: %s\n", q.Question, answer)
	}
	b.WriteString("Continue with these answers in mind.")
	return strings.TrimSpace(b.String())
}
