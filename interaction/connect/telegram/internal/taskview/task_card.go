package taskview

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"

	controlruntime "nekocode/runtime"
	"nekocode/runtime/view"
	"nekocode/util/text"
)

const (
	maxTelegramBody = 3600
	maxRunCards     = 20
)

type Tracker struct {
	mu               sync.Mutex
	runs             map[controlruntime.RunID]*taskCard
	order            []controlruntime.RunID
	last             controlruntime.RunID
	pendingQuestions map[string]pendingQuestion
	lastQuestionID   string
}

type pendingQuestion struct {
	View controlruntime.QuestionView
}

type taskCard struct {
	Title     string
	Status    string
	LastPhase string
	Tools     []toolCard
	Diffs     []string
	Result    string
	Error     string
}

type toolCard struct {
	Name   string
	Brief  string
	Status string
}

func NewTracker() *Tracker {
	return &Tracker{
		runs:             make(map[controlruntime.RunID]*taskCard),
		pendingQuestions: make(map[string]pendingQuestion),
	}
}

func (t *Tracker) RenderEvent(ev controlruntime.Event) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch ev.Type {
	case controlruntime.EventInputAccepted:
		if p, ok := ev.Payload.(controlruntime.MessagePayload); ok {
			card := t.card(ev.RunID)
			card.Title = firstLine(p.Content)
			card.Status = "accepted"
		}
	case controlruntime.EventRunStarted:
		card := t.card(ev.RunID)
		card.Status = "running"
	case controlruntime.EventPhaseChanged:
		if p, ok := ev.Payload.(controlruntime.PhasePayload); ok {
			card := t.card(ev.RunID)
			card.LastPhase = p.Phase
		}
	case controlruntime.EventToolStarted:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			tool := toolCard{Name: p.ToolName, Brief: toolBrief(p.ToolName, p.Args), Status: "running"}
			card.Tools = append(card.Tools, tool)
		}
	case controlruntime.EventToolPreview:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			if strings.TrimSpace(p.Preview) == "" {
				return ""
			}
			if isDiffLike(p.ToolName, p.Preview) {
				clean := cleanDiffPreview(p.Preview)
				card.Diffs = append(card.Diffs, clean)
				return renderDiffPreview(p.Preview)
			}
		}
	case controlruntime.EventToolCompleted, controlruntime.EventToolBlocked:
		if p, ok := ev.Payload.(controlruntime.ToolPayload); ok {
			card := t.card(ev.RunID)
			status := "done"
			if ev.Type == controlruntime.EventToolBlocked || p.IsError {
				status = "blocked"
			}
			t.finishTool(card, p, status)
			if status == "blocked" {
				return compactMessage(
					htmlTitle("Blocked"),
					labelCode("Tool", strings.TrimSpace(p.ToolName+" "+toolBrief(p.ToolName, p.Args))),
					htmlPre(truncateRunes(p.Output, 1200)),
				)
			}
		}
	case controlruntime.EventApprovalRequested:
		if p, ok := ev.Payload.(controlruntime.ApprovalView); ok {
			card := t.card(ev.RunID)
			card.Status = "waiting approval"
			return compactMessage(htmlTitle("Approval"), approvalSummary(p))
		}
	case controlruntime.EventQuestionRequested:
		if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
			card := t.card(ev.RunID)
			card.Status = "waiting question"
			t.pendingQuestions[p.ID] = pendingQuestion{View: p}
			t.lastQuestionID = p.ID
			return compactMessage(htmlTitle("Input"), questionSummary(p))
		}
	case controlruntime.EventApprovalResolved, controlruntime.EventQuestionResolved:
		card := t.card(ev.RunID)
		if card.Status == "waiting approval" || card.Status == "waiting question" {
			card.Status = "running"
		}
		if ev.Type == controlruntime.EventQuestionResolved {
			if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
				delete(t.pendingQuestions, p.ID)
			}
		}
	case controlruntime.EventRunDone:
		if p, ok := ev.Payload.(controlruntime.DonePayload); ok {
			card := t.card(ev.RunID)
			card.Status = "done"
			card.Result = p.Output
			return t.doneReplyLocked(card)
		}
	case controlruntime.EventRunFailed:
		if p, ok := ev.Payload.(controlruntime.DonePayload); ok {
			card := t.card(ev.RunID)
			card.Status = "failed"
			card.Result = p.Output
			card.Error = p.Error
			return t.doneReplyLocked(card)
		}
	case controlruntime.EventRunAborted:
		card := t.card(ev.RunID)
		card.Status = "aborted"
		return htmlTitle("Stopped")
	}
	return ""
}

func (t *Tracker) Status() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.order) == 0 {
		return compactMessage(htmlTitle("Status"), htmlEscape("Idle"), htmlEscape("Send a message here to start a task."))
	}
	card := t.runs[t.last]
	if card == nil {
		return compactMessage(htmlTitle("Status"), htmlEscape("Idle"))
	}
	return t.statusSummaryLocked(card)
}

func (t *Tracker) LastSummary() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.order) == 0 {
		return htmlEscape("No remote tasks yet.")
	}
	card := t.runs[t.last]
	if card == nil {
		return htmlEscape("No remote tasks yet.")
	}
	if card.Status == "done" || card.Status == "failed" {
		return t.lastSummaryLocked(card)
	}
	return t.cardStatusLocked(card)
}

func (t *Tracker) DiffSummary(runID controlruntime.RunID) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if runID == "" {
		runID = t.last
	}
	card := t.runs[runID]
	if card == nil {
		return htmlEscape("No matching task was found.")
	}
	if len(card.Diffs) == 0 {
		return htmlEscape("No diff preview is available yet.")
	}
	return compactMessage(htmlTitle("Diff"), htmlPre(truncateRunes(card.Diffs[len(card.Diffs)-1], 3000)))
}

func renderDiffPreview(preview string) string {
	path := diffPath(preview)
	clean := cleanDiffPreview(preview)
	if strings.TrimSpace(clean) == "" {
		return ""
	}
	if path == "" {
		path = diffPath(clean)
	}
	add, del := diffLineCounts(clean)
	title := "Diff"
	var meta []string
	if path != "" {
		meta = append(meta, path)
	}
	if add > 0 || del > 0 {
		meta = append(meta, fmt.Sprintf("+%d -%d", add, del))
	}
	if len(meta) > 0 {
		return compactMessage(htmlTitle(title), htmlCode(strings.Join(meta, "  ")), htmlPre(truncateRunes(clean, 2600)))
	}
	return compactMessage(htmlTitle(title), htmlPre(truncateRunes(clean, 2600)))
}

func cleanDiffPreview(preview string) string {
	lines := strings.Split(strings.TrimRight(preview, "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "EDIT_PREVIEW_JSON_B64 ") {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "---" {
				out = out[:len(out)-1]
			}
			break
		}
		if isInternalDiffHeader(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func isInternalDiffHeader(line string) bool {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return false
	}
	return strings.Contains(line, "#") || strings.HasPrefix(line, "[write ")
}

func diffPath(preview string) string {
	for _, line := range strings.Split(preview, "\n") {
		switch {
		case isInternalDiffHeader(line):
			header := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			if hash := strings.LastIndexByte(header, '#'); hash > 0 {
				return header[:hash]
			}
			if strings.HasPrefix(header, "write ") {
				return strings.TrimSpace(strings.TrimPrefix(header, "write "))
			}
		case strings.HasPrefix(line, "--- "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			path = strings.TrimPrefix(path, "a/")
			if path != "" && path != "/dev/null" {
				return path
			}
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			path = strings.TrimPrefix(path, "b/")
			if path != "" && path != "/dev/null" {
				return path
			}
		}
	}
	return ""
}

func diffLineCounts(preview string) (add, del int) {
	for _, line := range strings.Split(preview, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			add++
			continue
		}
		if strings.HasPrefix(line, "-") {
			del++
			continue
		}
		if colon := strings.IndexByte(line, ':'); colon > 0 {
			prefix := strings.TrimSpace(line[:colon])
			if strings.HasPrefix(prefix, "+") {
				add++
			} else if strings.HasPrefix(prefix, "-") {
				del++
			}
		}
	}
	return add, del
}

func (t *Tracker) BuildQuestionReply(questionID, raw string) (view.QuestionReply, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if strings.TrimSpace(questionID) == "" {
		questionID = t.lastQuestionID
	}
	pending, ok := t.pendingQuestions[questionID]
	if !ok {
		return view.QuestionReply{}, "", fmt.Errorf("question %s is not pending", questionID)
	}
	reply, err := buildQuestionReply(pending.View, raw)
	if err != nil {
		return view.QuestionReply{}, "", err
	}
	return reply, questionID, nil
}

func (t *Tracker) RejectQuestion(questionID string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if strings.TrimSpace(questionID) == "" {
		questionID = t.lastQuestionID
	}
	if _, ok := t.pendingQuestions[questionID]; !ok {
		return "", fmt.Errorf("question %s is not pending", questionID)
	}
	return questionID, nil
}

func (t *Tracker) BuildQuestionOptionReply(questionID string, optionIndex int) (view.QuestionReply, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pending, ok := t.pendingQuestions[questionID]
	if !ok {
		return view.QuestionReply{}, "", fmt.Errorf("question %s is not pending", questionID)
	}
	if len(pending.View.Questions) != 1 {
		return view.QuestionReply{}, "", fmt.Errorf("question %s requires text answer", questionID)
	}
	item := pending.View.Questions[0]
	if item.Multiple || optionIndex < 0 || optionIndex >= len(item.Options) {
		return view.QuestionReply{}, "", fmt.Errorf("question %s requires text answer", questionID)
	}
	return view.QuestionReply{Answers: [][]string{{item.Options[optionIndex].Label}}}, questionID, nil
}

func (t *Tracker) card(id controlruntime.RunID) *taskCard {
	if id == "" {
		id = "run_unknown"
	}
	if card, ok := t.runs[id]; ok {
		t.last = id
		return card
	}
	card := &taskCard{Status: "created"}
	t.runs[id] = card
	t.order = append(t.order, id)
	t.last = id
	if len(t.order) > maxRunCards {
		drop := t.order[0]
		t.order = t.order[1:]
		delete(t.runs, drop)
	}
	return card
}

func (t *Tracker) finishTool(card *taskCard, p controlruntime.ToolPayload, status string) {
	brief := toolBrief(p.ToolName, p.Args)
	for i := len(card.Tools) - 1; i >= 0; i-- {
		if card.Tools[i].Name == p.ToolName && card.Tools[i].Status == "running" {
			card.Tools[i].Brief = brief
			card.Tools[i].Status = status
			return
		}
	}
	card.Tools = append(card.Tools, toolCard{
		Name:   p.ToolName,
		Brief:  brief,
		Status: status,
	})
}

func (t *Tracker) doneReplyLocked(card *taskCard) string {
	if card.Status == "failed" {
		lines := []string{htmlTitle("Failed")}
		if card.Error != "" {
			lines = append(lines, "", htmlTitle("Error"), htmlPre(truncateRunes(card.Error, 1400)))
		}
		if strings.TrimSpace(card.Result) != "" {
			lines = append(lines, "", htmlTitle("Result"), htmlBody(card.Result, 1600))
		}
		return compactMessage(lines...)
	}

	lines := make([]string, 0, 3)
	if strings.TrimSpace(card.Result) != "" {
		lines = append(lines, htmlBody(card.Result, 1800))
	}
	lines = appendDiffShortcut(lines, card, true)
	return compactMessage(lines...)
}

func (t *Tracker) lastSummaryLocked(card *taskCard) string {
	title := "Done"
	if card.Status == "failed" {
		title = "Failed"
	}
	lines := []string{htmlTitle(title)}
	lines = append(lines, htmlEscape(compactCounts(card)))
	if card.Error != "" {
		lines = append(lines, "", htmlTitle("Error"), htmlPre(truncateRunes(card.Error, 1400)))
	}
	if strings.TrimSpace(card.Result) != "" {
		lines = append(lines, "", htmlTitle("Result"), htmlBody(card.Result, 1600))
	}
	lines = appendDiffShortcut(lines, card, true)
	return compactMessage(lines...)
}

func (t *Tracker) statusSummaryLocked(card *taskCard) string {
	lines := []string{htmlTitle("Status"), htmlEscape(statusTitle(card.Status))}
	lines = appendTaskMeta(lines, card)
	if card.Status == "waiting approval" {
		lines = append(lines, htmlEscape("Waiting for approval"))
	}
	if card.Status == "waiting question" {
		lines = append(lines, htmlEscape("Waiting for input"))
	}
	lines = appendDiffShortcut(lines, card, false)
	return compactMessage(lines...)
}

func (t *Tracker) cardStatusLocked(card *taskCard) string {
	lines := []string{htmlTitle(statusTitle(card.Status))}
	lines = appendTaskMeta(lines, card)
	if len(card.Tools) > 0 {
		start := len(card.Tools) - 5
		if start < 0 {
			start = 0
		}
		lines = append(lines, "", htmlTitle("Tools"))
		for _, tool := range card.Tools[start:] {
			lines = append(lines, "- "+htmlEscape(toolLine(tool)))
		}
	}
	lines = appendDiffShortcut(lines, card, len(card.Tools) > 0)
	return compactMessage(lines...)
}

func appendTaskMeta(lines []string, card *taskCard) []string {
	if card.Title != "" {
		lines = append(lines, labelText("Task", card.Title))
	}
	if card.LastPhase != "" {
		lines = append(lines, labelText("Phase", card.LastPhase))
	}
	return append(lines, htmlEscape(compactCounts(card)))
}

func appendDiffShortcut(lines []string, card *taskCard, leadingBlank bool) []string {
	if len(card.Diffs) == 0 {
		return lines
	}
	if leadingBlank {
		lines = append(lines, "")
	}
	return append(lines, labelCode("Diff", "/diff"))
}

func statusTitle(status string) string {
	switch status {
	case "accepted", "created":
		return "Queued"
	case "running":
		return "Working"
	case "waiting approval":
		return "Waiting for approval"
	case "waiting question":
		return "Waiting for your input"
	case "done":
		return "Done"
	case "failed":
		return "Failed"
	case "aborted":
		return "Stopped"
	default:
		if status == "" {
			return "Status"
		}
		return titleWords(status)
	}
}

func titleWords(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func compactCounts(card *taskCard) string {
	parts := []string{fmt.Sprintf("Steps: %d", len(card.Tools))}
	if len(card.Diffs) > 0 {
		parts = append(parts, fmt.Sprintf("Diff: %d", len(card.Diffs)))
	}
	return strings.Join(parts, " | ")
}

func approvalSummary(p controlruntime.ApprovalView) string {
	var b strings.Builder
	b.WriteString(labelCode("Tool", p.ToolName))
	if p.Kind != "" {
		fmt.Fprintf(&b, "\n%s", labelText("Kind", p.Kind))
	}
	if cmd, ok := stringArg(p.Args, "command"); ok && cmd != "" {
		fmt.Fprintf(&b, "\n%s", labelText("Command", ""))
		fmt.Fprintf(&b, "\n%s", htmlPre(truncateRunes(cmd, 900)))
		return b.String()
	}
	if path, ok := stringArg(p.Args, "path"); ok && path != "" {
		fmt.Fprintf(&b, "\n%s", labelCode("Path", path))
	}
	if summary, ok := stringArg(p.Args, "summary"); ok && summary != "" {
		fmt.Fprintf(&b, "\n%s", htmlEscape(truncateRunes(summary, 900)))
	}
	if preview, ok := stringArg(p.Args, "_preview"); ok && preview != "" {
		fmt.Fprintf(&b, "\n%s", labelText("Preview", ""))
		fmt.Fprintf(&b, "\n%s", htmlPre(truncateRunes(preview, 1600)))
	}
	return b.String()
}

func questionSummary(p controlruntime.QuestionView) string {
	if len(p.Questions) == 0 {
		return compactMessage(htmlEscape("NekoCode requested input."), labelCode("Reply", "/answer "+p.ID+" <answer>"))
	}
	if usesQuestionButtons(p) {
		q := p.Questions[0]
		header := strings.TrimSpace(q.Header)
		if header == "" {
			header = "Question"
		}
		return compactMessage(htmlTitle(header), htmlEscape(q.Question))
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
		fmt.Fprintf(&b, "%s\n%s", htmlTitle(header), htmlEscape(q.Question))
		if len(q.Options) > 0 {
			b.WriteString("\n")
			for idx, opt := range q.Options {
				fmt.Fprintf(&b, "\n%d. %s", idx+1, htmlEscape(opt.Label))
				if opt.Description != "" {
					b.WriteString(": ")
					b.WriteString(htmlEscape(opt.Description))
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

func Help() string {
	return strings.Join([]string{
		htmlTitle("Commands"),
		labelCode("Status", "/status"),
		labelCode("Last", "/last"),
		labelCode("Diff", "/diff"),
		labelCode("Stop", "/stop"),
		"",
		htmlEscape("Use buttons for approvals and single-choice questions."),
	}, "\n")
}

func answerCommand(questionID string) string {
	return "/answer " + questionID + " <answer>"
}

func dismissCommand(questionID string) string {
	return "/dismiss " + questionID
}

func buildQuestionReply(qv controlruntime.QuestionView, raw string) (view.QuestionReply, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return view.QuestionReply{}, fmt.Errorf("empty answer")
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
			return view.QuestionReply{}, fmt.Errorf("question %d: %w", i+1, err)
		}
		answers[i] = parsed
	}
	return view.QuestionReply{Answers: answers}, nil
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

func parseQuestionAnswer(item view.QuestionItem, raw string) ([]string, error) {
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

func matchQuestionOption(options []view.QuestionOption, token string) (string, bool) {
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

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	line := strings.SplitN(s, "\n", 2)[0]
	return text.TruncateByRune(line, 120)
}

func compactMessage(parts ...string) string {
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}
			continue
		}
		lines = append(lines, part)
	}
	out := strings.TrimSpace(strings.Join(lines, "\n"))
	if out == "" {
		return ""
	}
	return truncateRunes(out, maxTelegramBody)
}

func htmlTitle(s string) string {
	return "<b>" + htmlEscape(s) + "</b>"
}

func htmlCode(s string) string {
	return "<code>" + htmlEscape(s) + "</code>"
}

func htmlPre(s string) string {
	return "<pre>" + htmlEscape(s) + "</pre>"
}

func htmlBody(s string, max int) string {
	return htmlEscape(truncateRunes(strings.TrimSpace(s), max))
}

func HTMLEscape(s string) string {
	return htmlEscape(s)
}

func htmlEscape(s string) string {
	return html.EscapeString(s)
}

func labelText(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return htmlEscape(label + ":")
	}
	return htmlEscape(label + ": " + value)
}

func labelCode(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return htmlEscape(label + ":")
	}
	return htmlEscape(label+": ") + htmlCode(value)
}

func toolLine(tool toolCard) string {
	brief := strings.TrimSpace(tool.Brief)
	if brief == "" {
		return strings.TrimSpace(tool.Name + " " + tool.Status)
	}
	return strings.TrimSpace(tool.Name + " " + tool.Status + " · " + brief)
}

func isDiffLike(toolName, preview string) bool {
	if toolName == "edit" || toolName == "write" || toolName == "diff" {
		return true
	}
	return strings.Contains(preview, "\n---") && strings.Contains(preview, "\n+++")
}

func toolBrief(toolName, raw string) string {
	args := parseToolArgs(raw)
	switch toolName {
	case "read", "write", "list", "tree", "edit":
		return args["path"]
	case "shell":
		return shellBrief(args)
	case "glob":
		return args["pattern"]
	case "grep":
		pattern := args["pattern"]
		if path := args["path"]; path != "" {
			return strings.TrimSpace(pattern + " " + path)
		}
		return pattern
	case "web_search", "web_fetch":
		if q := args["query"]; q != "" {
			return text.TruncateByRune(q, 80)
		}
		return text.TruncateByRune(args["url"], 80)
	case "task":
		typ := args["type"]
		if typ == "" {
			typ = "executor"
		}
		if desc := args["description"]; desc != "" {
			return text.TruncateByRune(typ+" · "+desc, 100)
		}
		return text.TruncateByRune(typ+" · "+firstLine(args["prompt"]), 100)
	default:
		for _, v := range args {
			return text.TruncateByRune(v, 80)
		}
		return ""
	}
}

func shellBrief(args map[string]string) string {
	action := strings.ToLower(strings.TrimSpace(args["action"]))
	if action == "" || action == "run" {
		return cleanArg(args["command"])
	}
	if action == "logs" {
		action = "poll"
	}
	switch action {
	case "list":
		return "shell sessions"
	case "wait", "poll", "stop":
		id := args["session_id"]
		if id == "" {
			id = args["id"]
		}
		if id != "" {
			return "session " + id
		}
		return "shell session"
	default:
		return action + " shell"
	}
}

func cleanArg(s string) string {
	s = strings.TrimSpace(s)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}
	return text.TruncateByRune(strings.Join(strings.Fields(s), " "), 120)
}

func parseToolArgs(s string) map[string]string {
	m := make(map[string]string)
	if strings.HasPrefix(strings.TrimSpace(s), "{") {
		var raw map[string]any
		if err := json.Unmarshal([]byte(s), &raw); err == nil {
			for k, v := range raw {
				switch t := v.(type) {
				case string:
					m[k] = t
				default:
					m[k] = fmt.Sprint(t)
				}
			}
			return m
		}
	}
	for _, pair := range text.SplitPairs(s) {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

func stringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		return fmt.Sprint(t), true
	}
}

func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func truncateRunes(s string, max int) string {
	return TruncateRunes(s, max)
}
