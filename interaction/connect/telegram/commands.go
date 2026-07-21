package telegram

import (
	"context"
	"strings"
)

const (
	usageAddProfile    = "Usage: /connect telegram add <bot-token>"
	usageUseProfile    = "Usage: /connect telegram use <name>"
	usageRemoveProfile = "Usage: /connect telegram remove <name>"
)

func (c *Connector) HandleCommand(ctx context.Context, args []string) (string, error) {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "add", "token":
			return c.addProfile(ctx, args[1:])
		case "profiles", "list":
			return c.profiles()
		case "use":
			return c.useProfile(ctx, args[1:])
		case "pair":
			return c.pairProfile(ctx, args[1:])
		case "unpair":
			return c.unpairProfile(args[1:])
		case "remove", "delete":
			return c.removeProfile(args[1:])
		case "reset":
			return c.resetConfig()
		case "status":
			return c.status()
		case "disconnect", "stop":
			if err := c.Stop(); err != nil {
				return "", err
			}
			return "Telegram connector stopped.", nil
		}
	}
	return c.connectActive(ctx)
}
