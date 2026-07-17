package defaultbot

import (
	"nekocode/bot"
	controlruntime "nekocode/runtime"
)

func NewSessionRuntime() *controlruntime.SessionRuntime {
	rt := controlruntime.NewSessionRuntime(bot.New())
	_ = rt.EnableEventRecording("")
	return rt
}
