package core

import (
	"context"
	"fmt"

	"nekocode/bot/command"
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
