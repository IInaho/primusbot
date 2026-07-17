// helpers.go — 工具参数/结果格式化辅助函数。
package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"nekocode/runtime/view"
	"nekocode/util/text"
)

func formatBriefArgs(toolName, toolArgs string) string {
	args := parseToolArgs(toolArgs)

	switch toolName {
	case "read":
		p := args["path"]
		if s, ok := args["startLine"]; ok {
			if e, ok2 := args["endLine"]; ok2 {
				return fmt.Sprintf("%s %s-%s", p, s, e)
			}
		}
		return p
	case "write", "list", "tree", "edit":
		return args["path"]
	case "shell":
		return formatShellArgs(args)
	case "glob":
		return args["pattern"]
	case "grep":
		p := args["path"]
		if p != "" {
			return args["pattern"] + " " + p
		}
		return args["pattern"]
	case "web_search", "web_fetch":
		q := args["query"]
		if q == "" {
			q = args["url"]
		}
		return text.TruncateByRune(q, 60)
	case "todo_write":
		return formatTodos(args["todos"])
	case "task":
		t := args["type"]
		if t == "" {
			t = "executor"
		}
		if d := args["description"]; d != "" {
			return t + " \u00b7 " + d
		}
		p := strings.SplitN(args["prompt"], "\n", 2)[0]
		p = strings.Trim(p, " \"")
		return t + " \u00b7 " + text.TruncateByRune(p, 30)
	default:
		for _, v := range args {
			return text.TruncateByRune(v, 50)
		}
		return ""
	}
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

func toolAction(toolName, toolArgs string) string {
	if toolName != "shell" {
		return ""
	}
	action := strings.ToLower(strings.TrimSpace(parseToolArgs(toolArgs)["action"]))
	if action == "logs" {
		return "poll"
	}
	return action
}

func formatShellArgs(args map[string]string) string {
	action := strings.ToLower(strings.TrimSpace(args["action"]))
	if action == "" || action == "run" {
		return cleanShellPreview(args["command"])
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

func cleanShellPreview(command string) string {
	command = strings.TrimSpace(command)
	if unquoted, err := strconv.Unquote(command); err == nil {
		return unquoted
	}
	replacer := strings.NewReplacer(`\"`, `"`, `\\`, `\`)
	return replacer.Replace(command)
}

func tokensSummary(stats view.BotStats) string {
	st := stats
	return "↑" + text.FormatTokens(st.TurnPrompt) + " ↓" + text.FormatTokens(st.TurnCompletion)
}

func formatTodos(raw string) string {
	if raw == "" {
		return ""
	}
	var items []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return ""
	}
	if len(items) == 0 {
		return ""
	}
	return fmt.Sprintf("%d items", len(items))
}
