package command

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"nekocode/protocol"
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
type MenuFunc func(ctx context.Context, cmd *Command) (protocol.CommandMenu, bool)

type commandKey struct {
	Prefix string
	Name   string
}

type commandEntry struct {
	DisplayName string
	Description string
	Handler     HandlerFunc
	Menu        MenuFunc
}

type Parser struct {
	handlers map[commandKey]commandEntry
}

func NewParser() *Parser {
	return &Parser{handlers: make(map[commandKey]commandEntry)}
}

func (p *Parser) Register(name string, handler HandlerFunc) {
	p.RegisterInfo(name, "", handler)
}

// RegisterInfo registers a slash command and the short description shown by
// transport-neutral command pickers.
func (p *Parser) RegisterInfo(name, description string, handler HandlerFunc) {
	p.RegisterWithPrefix(SlashPrefix, name, description, handler)
}

func (p *Parser) RegisterDynamic(name string, handler HandlerFunc) {
	p.RegisterDynamicInfo(name, "", handler)
}

func (p *Parser) RegisterDynamicInfo(name, description string, handler HandlerFunc) {
	p.RegisterWithPrefix(DollarPrefix, name, description, handler)
}

func (p *Parser) RegisterWithPrefix(prefix, name, description string, handler HandlerFunc) {
	prefix = normalizePrefix(prefix)
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		return
	}
	keyName := strings.ToLower(displayName)
	key := commandKey{Prefix: prefix, Name: keyName}
	entry := p.handlers[key]
	entry.DisplayName = displayName
	entry.Description = strings.TrimSpace(description)
	entry.Handler = handler
	p.handlers[key] = entry
}

// RegisterMenu adds a dynamic picker to an existing slash command. Commands
// without a finite set of choices keep their normal text behavior.
func (p *Parser) RegisterMenu(name string, menu MenuFunc) {
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		return
	}
	key := commandKey{Prefix: SlashPrefix, Name: strings.ToLower(displayName)}
	entry, exists := p.handlers[key]
	if !exists {
		return
	}
	entry.Menu = menu
	p.handlers[key] = entry
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

// RootMenu returns the registered commands under one prefix. Root choices
// never auto-submit: selecting one either expands its menu or fills the input
// for an explicit second confirmation.
func (p *Parser) RootMenu(prefix string) protocol.CommandMenu {
	prefix = normalizePrefix(prefix)
	items := make([]protocol.CommandMenuItem, 0, len(p.handlers))
	for key, entry := range p.handlers {
		if key.Prefix != prefix {
			continue
		}
		items = append(items, protocol.CommandMenuItem{
			Value:       prefix + entry.DisplayName,
			Label:       prefix + entry.DisplayName,
			Description: entry.Description,
			Submit:      false,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	title := "Commands"
	if prefix == DollarPrefix {
		title = "Skills"
	}
	return protocol.CommandMenu{Title: title, Empty: "No commands available", Items: items}
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

// Menu resolves the next finite set of choices for the current command input.
func (p *Parser) Menu(ctx context.Context, input string) (protocol.CommandMenu, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == SlashPrefix || trimmed == DollarPrefix {
		return p.RootMenu(trimmed), true
	}
	cmd := p.Parse(input)
	if cmd.Name == "" {
		return protocol.CommandMenu{}, false
	}
	if cmd.Prefix == SlashPrefix && cmd.Name == "help" && len(cmd.Args) == 0 {
		return p.RootMenu(SlashPrefix), true
	}
	entry, exists := p.handlers[commandKey{Prefix: normalizePrefix(cmd.Prefix), Name: cmd.Name}]
	if !exists || entry.Menu == nil || ctx.Err() != nil {
		return protocol.CommandMenu{}, false
	}
	return entry.Menu(ctx, cmd)
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

	p.RegisterInfo("help", "Show available commands", func(_ context.Context, cmd *Command) (string, bool) {
		return formatCommandHelp(p.RootMenu(SlashPrefix), p.RootMenu(DollarPrefix)), true
	})

	p.RegisterInfo("clear", "Clear conversation history", func(_ context.Context, cmd *Command) (string, bool) {
		if deps.ResetConversation == nil {
			return "Conversation reset is unavailable.", true
		}
		result, err := deps.ResetConversation(false)
		if err != nil {
			return "Failed to clear conversation: " + err.Error(), true
		}
		return result, true
	})

	p.RegisterInfo("context", "Show context usage", func(_ context.Context, cmd *Command) (string, bool) {
		return ContextReport(deps.CtxMgr, deps.ToolRegistry.Descriptors()), true
	})

	p.RegisterInfo("summarize", "Compress context now", func(_ context.Context, cmd *Command) (string, bool) {
		result, err := ForceSummarize(deps.CtxMgr, true)
		if err != nil {
			return "Summarize failed: " + err.Error(), true
		}
		return result, true
	})

	p.RegisterInfo("new", "Start a new conversation", func(_ context.Context, cmd *Command) (string, bool) {
		if deps.ResetConversation == nil {
			return "Conversation reset is unavailable.", true
		}
		result, err := deps.ResetConversation(true)
		if err != nil {
			return "Failed to start new conversation: " + err.Error(), true
		}
		return result, true
	})

	p.RegisterInfo("rewind", "Restore files to a checkpoint", func(_ context.Context, cmd *Command) (string, bool) {
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

	p.RegisterInfo("config", "Show the active model", func(_ context.Context, cmd *Command) (string, bool) {
		return getConfig(), true
	})

	p.RegisterInfo("model", "Choose the active model", func(_ context.Context, cmd *Command) (string, bool) {
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
		if err := deps.SwitchModel(strings.Join(cmd.Args, " ")); err != nil {
			return err.Error(), true
		}
		return "Switched to " + getConfig(), true
	})
}

func formatCommandHelp(menus ...protocol.CommandMenu) string {
	var b strings.Builder
	for _, menu := range menus {
		if len(menu.Items) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(menu.Title)
		b.WriteString(":\n")
		for _, item := range menu.Items {
			fmt.Fprintf(&b, "  %-14s", item.Label)
			if item.Description != "" {
				b.WriteString(item.Description)
			}
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}
