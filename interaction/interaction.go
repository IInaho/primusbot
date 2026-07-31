// Package interaction contains rendering-neutral behavior shared by
// interaction surfaces.
package interaction

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"nekocode/util/text"
)

// ToolBrief returns a short human-readable summary of a tool call's
// arguments, suitable for rendering next to the tool name.
func ToolBrief(toolName, rawArgs string) string {
	args := parseToolArgs(rawArgs)
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
		return shellBrief(args)
	case "glob":
		return args["pattern"]
	case "grep":
		if p := args["path"]; p != "" {
			return strings.TrimSpace(args["pattern"] + " " + p)
		}
		return args["pattern"]
	case "web_search", "web_fetch":
		q := args["query"]
		if q == "" {
			q = args["url"]
		}
		return text.TruncateByRune(q, 60)
	case "todo_write":
		return todosBrief(args["todos"])
	case "task":
		typ := args["type"]
		if typ == "" {
			typ = "executor"
		}
		if d := args["description"]; d != "" {
			return typ + " · " + d
		}
		p := strings.SplitN(args["prompt"], "\n", 2)[0]
		p = strings.Trim(p, " \"")
		return typ + " · " + text.TruncateByRune(p, 30)
	default:
		for _, v := range args {
			return text.TruncateByRune(v, 50)
		}
		return ""
	}
}

// ToolAction returns the normalized sub-action of a tool call, or "" for
// tools without actions. The legacy shell "logs" action maps to "poll".
func ToolAction(toolName, rawArgs string) string {
	if toolName != "shell" {
		return ""
	}
	return shellAction(parseToolArgs(rawArgs))
}

func shellAction(args map[string]string) string {
	action := strings.ToLower(strings.TrimSpace(args["action"]))
	if action == "logs" {
		return "poll"
	}
	return action
}

func shellBrief(args map[string]string) string {
	action := shellAction(args)
	if action == "" || action == "run" {
		return cleanShellCommand(args["command"])
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

func cleanShellCommand(command string) string {
	command = strings.TrimSpace(command)
	if unquoted, err := strconv.Unquote(command); err == nil {
		return unquoted
	}
	replacer := strings.NewReplacer(`\"`, `"`, `\\`, `\`)
	return replacer.Replace(command)
}

func todosBrief(raw string) string {
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

// parseToolArgs parses tool arguments encoded either as a JSON object or as
// comma-separated key=value pairs.
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
