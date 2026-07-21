package defaultbot

import (
	"nekocode/bot"
	"nekocode/interaction/connect/telegram"
	controlruntime "nekocode/runtime"
)

func NewSessionRuntime() *controlruntime.SessionRuntime {
	rt := controlruntime.NewSessionRuntimeWithCoreOptions(coreOptions(bot.New()))
	_ = rt.EnableEventRecording("")
	return rt
}

func NewSessionRuntimeWithTelegram() *controlruntime.SessionRuntime {
	rt := NewSessionRuntime()
	rt.RegisterConnector("telegram", func(runtime controlruntime.Runtime) controlruntime.Connector {
		return telegram.New(runtime)
	})
	return rt
}
