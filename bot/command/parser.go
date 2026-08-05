package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	SlashPrefix  = "/"
	DollarPrefix = "$"
)

type Command struct {
	Prefix string
	Name   string
	Args   []string
	Raw    string
}

type HandlerFunc func(ctx context.Context, cmd *Command) (string, bool)

type commandKey struct {
	Prefix string
	Name   string
}

type commandEntry struct {
	DisplayName string
	Handler     HandlerFunc
}

type Parser struct {
	handlers map[commandKey]commandEntry
}

func NewParser() *Parser {
	return &Parser{handlers: make(map[commandKey]commandEntry)}
}

func (p *Parser) Register(name string, handler HandlerFunc) {
	p.RegisterWithPrefix(SlashPrefix, name, handler)
}

func (p *Parser) RegisterDynamic(name string, handler HandlerFunc) {
	p.RegisterWithPrefix(DollarPrefix, name, handler)
}

func (p *Parser) RegisterWithPrefix(prefix, name string, handler HandlerFunc) {
	prefix = normalizePrefix(prefix)
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		return
	}
	keyName := strings.ToLower(displayName)
	p.handlers[commandKey{Prefix: prefix, Name: keyName}] = commandEntry{DisplayName: displayName, Handler: handler}
}

// ClearPrefix removes every command registered under one prefix.
func (p *Parser) ClearPrefix(prefix string) {
	prefix = normalizePrefix(prefix)
	for key := range p.handlers {
		if key.Prefix == prefix {
			delete(p.handlers, key)
		}
	}
}

func (p *Parser) Commands() []string {
	names := make([]string, 0, len(p.handlers))
	for key, entry := range p.handlers {
		names = append(names, key.Prefix+entry.DisplayName)
	}
	sort.Strings(names)
	return names
}

func (p *Parser) Parse(input string) *Command {
	trimmed := strings.TrimSpace(input)
	prefix := commandPrefix(trimmed)
	if prefix == "" {
		return &Command{Name: "", Raw: input}
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return &Command{Name: "", Raw: input}
	}
	name := strings.ToLower(strings.TrimPrefix(parts[0], prefix))
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}
	return &Command{Prefix: prefix, Name: name, Args: args, Raw: input}
}

func (p *Parser) Execute(ctx context.Context, cmd *Command) (string, bool) {
	if cmd.Name == "" {
		return "", false
	}
	prefix := normalizePrefix(cmd.Prefix)
	name := strings.ToLower(strings.TrimSpace(cmd.Name))
	entry, exists := p.handlers[commandKey{Prefix: prefix, Name: name}]
	if !exists {
		return "Unknown command: " + prefix + name + ". Type /help for available commands.", true
	}
	if err := ctx.Err(); err != nil {
		return "Command cancelled: " + err.Error(), true
	}
	return entry.Handler(ctx, cmd)
}

func normalizePrefix(prefix string) string {
	if prefix == DollarPrefix {
		return DollarPrefix
	}
	return SlashPrefix
}

func commandPrefix(input string) string {
	if strings.HasPrefix(input, SlashPrefix) {
		return SlashPrefix
	}
	if strings.HasPrefix(input, DollarPrefix) {
		return DollarPrefix
	}
	return ""
}

// RegisterDefaults registers the built-in slash commands using Deps directly.
func RegisterDefaults(p *Parser, deps Deps) {
	getConfig := func() string {
		selection := deps.GetConfigFn()
		return selection.Provider + "/" + selection.Model
	}

	p.Register("help", func(_ context.Context, cmd *Command) (string, bool) {
		return `Built-in slash commands:
  /help        Show this help message
  /new         Start a new conversation (keeps summary)
  /clear       Clear all conversation history
  /context     Show context window breakdown (bar + used/total + detail)
  /summarize   Force context compression now
  /rewind      Restore files or list checkpoints (/rewind [turn|list])
  /config      Show current provider and model
  /model       List or switch models (/model <name>)
  /plan        Read-only exploration mode, approve before execution
  /plugin      Manage plugins (install, list, uninstall, etc.)
  /sessions    Manage saved sessions
  /export      Export conversation context to JSON file

Dynamic dollar commands:
  $<skill>     Load a dynamically registered skill
`, true
	})

	p.Register("clear", func(_ context.Context, cmd *Command) (string, bool) {
		if deps.ResetConversation == nil {
			return "Conversation reset is unavailable.", true
		}
		result, err := deps.ResetConversation(false)
		if err != nil {
			return "Failed to clear conversation: " + err.Error(), true
		}
		return result, true
	})

	p.Register("context", func(_ context.Context, cmd *Command) (string, bool) {
		return ContextReport(deps.CtxMgr, deps.ToolRegistry.Descriptors()), true
	})

	p.Register("summarize", func(_ context.Context, cmd *Command) (string, bool) {
		result, err := ForceSummarize(deps.CtxMgr, true)
		if err != nil {
			return "Summarize failed: " + err.Error(), true
		}
		return result, true
	})

	p.Register("new", func(_ context.Context, cmd *Command) (string, bool) {
		if deps.ResetConversation == nil {
			return "Conversation reset is unavailable.", true
		}
		result, err := deps.ResetConversation(true)
		if err != nil {
			return "Failed to start new conversation: " + err.Error(), true
		}
		return result, true
	})

	p.Register("rewind", func(_ context.Context, cmd *Command) (string, bool) {
		if deps.Rewind == nil {
			return "Checkpoint rewind is unavailable.", true
		}
		if len(cmd.Args) > 1 {
			return "Usage: /rewind [turn|list]", true
		}
		turn := ""
		if len(cmd.Args) == 1 {
			turn = cmd.Args[0]
		}
		result, err := deps.Rewind(turn)
		if err != nil {
			return "Rewind failed: " + err.Error(), true
		}
		return result, true
	})

	p.Register("config", func(_ context.Context, cmd *Command) (string, bool) {
		return getConfig(), true
	})

	p.Register("model", func(_ context.Context, cmd *Command) (string, bool) {
		if len(cmd.Args) == 0 {
			var sb strings.Builder
			fmt.Fprintf(&sb, "Current: %s\n", getConfig())
			if deps.ListModelsFn != nil {
				names := deps.ListModelsFn()
				sb.WriteString("Available:\n")
				for _, n := range names {
					fmt.Fprintf(&sb, "  %s\n", n)
				}
			}
			sb.WriteString("\n/model <name> to switch")
			return sb.String(), true
		}
		if err := deps.SwitchModel(cmd.Args[0]); err != nil {
			return err.Error(), true
		}
		return "Switched to " + getConfig(), true
	})
}
