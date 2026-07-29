package defaultbot

import (
	"nekocode/bot"
	"nekocode/interaction/connect/feishu"
	"nekocode/interaction/connect/qqbot"
	"nekocode/interaction/connect/telegram"
	controlruntime "nekocode/runtime"
)

func NewSessionRuntime() *controlruntime.SessionRuntime {
	rt := controlruntime.NewSessionRuntimeWithCoreOptions(coreOptions(bot.New()))
	_ = rt.EnableEventRecording("")
	return rt
}

// NewSessionRuntimeWithConnectors assembles the runtime with all IM
// connectors (telegram, feishu, qqbot) registered.
func NewSessionRuntimeWithConnectors() *controlruntime.SessionRuntime {
	rt := NewSessionRuntime()
	rt.RegisterConnector("telegram", func(runtime controlruntime.Runtime) controlruntime.Connector {
		return telegram.New(runtime)
	})
	rt.RegisterConnector("feishu", func(runtime controlruntime.Runtime) controlruntime.Connector {
		return feishu.New(runtime)
	})
	rt.RegisterConnector("qqbot", func(runtime controlruntime.Runtime) controlruntime.Connector {
		return qqbot.New(runtime)
	})
	return rt
}
