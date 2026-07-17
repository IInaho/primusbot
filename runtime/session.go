package runtime

import (
	"nekocode/runtime/internal/botcore"
	"nekocode/runtime/internal/session"
)

type RuntimeBot = botcore.RuntimeBot
type GUIBot = botcore.GUIBot

type SessionRuntime struct {
	*session.SessionRuntime
	bot RuntimeBot
}

func NewSessionRuntime(b RuntimeBot) *SessionRuntime {
	return &SessionRuntime{SessionRuntime: session.NewSessionRuntime(b), bot: b}
}
