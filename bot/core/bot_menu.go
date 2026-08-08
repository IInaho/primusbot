package core

import (
	"context"
	"fmt"

	"nekocode/bot/command"
	"nekocode/bot/config"
	"nekocode/protocol"
)

func (b *Bot) registerCommandMenus(p *command.Parser) {
	p.RegisterMenu("model", func(_ context.Context, cmd *command.Command) (protocol.CommandMenu, bool) {
		if len(cmd.Args) != 0 {
			return protocol.CommandMenu{}, false
		}
		active := b.cfg.Active
		items := make([]protocol.CommandMenuItem, 0, len(b.cfg.Models))
		for _, model := range b.cfg.Models {
			description := model.Provider + " / " + model.Model
			if model.Name == active {
				description += " · current"
			}
			items = append(items, protocol.CommandMenuItem{
				Value: "/model " + model.Name, Label: model.Name,
				Description: description, Submit: true,
			})
		}
		return protocol.CommandMenu{Title: "Choose model", Empty: "No models configured", Items: items}, true
	})

	p.RegisterMenu("permission", func(_ context.Context, cmd *command.Command) (protocol.CommandMenu, bool) {
		if len(cmd.Args) != 0 {
			return protocol.CommandMenu{}, false
		}
		full := b.fullAccess.Load()
		manualDesc := "Prompt for approval on guarded commands (default)"
		fullDesc := "Run ALL commands with no approval — DANGEROUS"
		if full {
			fullDesc += " · current"
		} else {
			manualDesc += " · current"
		}
		return protocol.CommandMenu{Title: "Permission mode", Items: []protocol.CommandMenuItem{
			{Value: "/permission manual", Label: "manual", Description: manualDesc, Submit: true},
			{Value: "/permission full", Label: "full (全接管)", Description: fullDesc, Submit: true},
		}}, true
	})

	p.RegisterMenu("effort", func(_ context.Context, cmd *command.Command) (protocol.CommandMenu, bool) {
		if len(cmd.Args) != 0 {
			return protocol.CommandMenu{}, false
		}
		model := b.cfg.ActiveModelConfig()
		current := model.ReasoningEffort
		levels := config.ReasoningCapabilityFor(model).Values()
		currentValue := current
		if currentValue == "" {
			currentValue = "auto"
		}
		items := make([]protocol.CommandMenuItem, 0, len(levels))
		for _, level := range levels {
			description := reasoningEffortDescription(level)
			if level == currentValue {
				description += " · current"
			}
			label := level
			switch level {
			case "auto":
				label = "Auto"
			case "none":
				label = "Off"
			}
			items = append(items, protocol.CommandMenuItem{
				Value: "/effort " + level, Label: label,
				Description: description, Submit: true,
			})
		}
		return protocol.CommandMenu{Title: "Reasoning effort", Items: items}, true
	})

	p.RegisterMenu("rewind", func(_ context.Context, cmd *command.Command) (protocol.CommandMenu, bool) {
		if len(cmd.Args) != 0 || b.checkpoints == nil || b.sess == nil {
			return protocol.CommandMenu{}, false
		}
		history, err := b.checkpoints.History(b.sess.CurrentID())
		if err != nil {
			return protocol.CommandMenu{Title: "Rewind checkpoint", Empty: err.Error()}, true
		}
		items := make([]protocol.CommandMenuItem, 0, len(history))
		for _, turn := range history {
			created, modified, deleted := checkpointChangeCounts(turn)
			items = append(items, protocol.CommandMenuItem{
				Value: "/rewind " + turn.Turn, Label: "Turn " + turn.Turn,
				Description: fmt.Sprintf("%s · %d files · +%d ~%d -%d",
					turn.CreatedAt.Local().Format("01-02 15:04"), len(turn.Changes), created, modified, deleted),
				Submit: true,
			})
		}
		return protocol.CommandMenu{Title: "Rewind checkpoint", Empty: "No checkpoints available", Items: items}, true
	})
}

func reasoningEffortDescription(effort string) string {
	switch effort {
	case "auto":
		return "Use the provider/model default"
	case "none":
		return "Disable reasoning when supported"
	case "minimal":
		return "Minimum reasoning budget"
	case "low":
		return "Faster, lighter reasoning"
	case "medium":
		return "Balanced reasoning"
	case "high":
		return "Deeper reasoning"
	case "xhigh":
		return "Extra-high reasoning when supported"
	default:
		return "Maximum reasoning when supported"
	}
}
