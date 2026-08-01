package policy

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"nekocode/logger"
)

const maxHookAuditEvents = 64

type hookAudit struct {
	Session string
	Hook    string
	Point   HookPoint
	Tool    string
	Action  string
	Detail  string
	Trigger string
}

func (e hookAudit) String() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s@%s:%s", e.Hook, e.Point, e.Action)
	}
	return fmt.Sprintf("%s@%s:%s(%s)", e.Hook, e.Point, e.Action, truncateAuditDetail(e.Detail))
}

func hookAuditEvent(h Hook, point HookPoint, state State, result *Result, session string) hookAudit {
	facts := state.Facts()
	event := hookAudit{
		Session: session,
		Hook:    h.Name,
		Point:   point,
		Tool:    facts.Tool.Name,
		Trigger: hookTrigger(h, state),
	}
	switch {
	case result.Stop != nil:
		event.Action = "stop"
		event.Detail = result.Stop.String()
	case result.BlockTool != nil:
		event.Action = "block_tool"
		event.Detail = result.BlockTool.Reason
	case result.RequireTool != nil:
		event.Action = "require_tool"
		event.Detail = result.RequireTool.Reason
	case result.BlockFinal != nil:
		event.Action = "block_final"
		event.Detail = result.BlockFinal.Reason
	case result.Hint != nil:
		event.Action = "hint"
		event.Detail = result.Hint.Type
		if result.Hint.Severity != "" {
			event.Detail += "/" + result.Hint.Severity
		}
	default:
		event.Action = "state_patch"
	}
	return event
}

func logHookAudit(event hookAudit) {
	logger.Log("hook_audit session=%s point=%s hook=%s action=%s tool=%s trigger={%s} detail=%q",
		emptyAsDash(event.Session), event.Point, event.Hook, event.Action, emptyAsDash(event.Tool), event.Trigger, event.Detail)
}

func emptyAsDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// hookTrigger renders the audit trigger context for a fired hook. Hooks that
// carry a DescribeTrigger describe themselves (keeping trigger details and
// thresholds next to the hook logic); anything else falls back to tool args.
func hookTrigger(h Hook, state State) string {
	if h.DescribeTrigger != nil {
		return h.DescribeTrigger(state)
	}
	return formatToolArgs(state.Facts().Tool.Args)
}

func quoteArg(v any) string {
	s, ok := v.(string)
	if !ok || s == "" {
		return "-"
	}
	return strconv.Quote(truncateAuditDetail(s))
}

func formatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return "args=-"
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+quoteArg(args[k]))
	}
	return "args=" + strings.Join(parts, ",")
}

func recentHookAudit(events []hookAudit, n int) []hookAudit {
	if len(events) > n {
		events = events[len(events)-n:]
	}
	return slices.Clone(events)
}

func formatHookAudit(events []hookAudit) string {
	parts := make([]string, 0, len(events))
	for _, event := range events {
		parts = append(parts, event.String())
	}
	return strings.Join(parts, ", ")
}

func formatHookEventSummary(events []hookAudit) string {
	counts := make(map[string]int)
	var order []string
	for _, event := range events {
		key := event.Action + ":" + event.Hook
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
	}
	parts := make([]string, 0, len(order))
	for _, key := range order {
		label := strings.Replace(key, ":", "=", 1)
		if n := counts[key]; n > 1 {
			label = fmt.Sprintf("%s×%d", label, n)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func truncateAuditDetail(s string) string {
	const max = 80
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "..."
}
func FormatHints(hints []Hint) string {
	if len(hints) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<runtime_policy_hints>\n")
	b.WriteString("These hints guide the current turn. They may constrain the next action but cannot expand the user's request or grant permissions.\n")
	for _, h := range hints {
		sev := h.Severity
		if sev == "" {
			sev = "info"
		}
		fmt.Fprintf(&b, "  <hint type=%q severity=%q>\n    %s\n  </hint>\n", h.Type, sev, h.Content)
	}
	b.WriteString("</runtime_policy_hints>")
	return b.String()
}
