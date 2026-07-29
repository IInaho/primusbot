// Package standard assembles NekoCode's standard runtime with the default bot,
// event recording and bundled connectors.
package standard

import (
	"fmt"

	"nekocode/bot"
	"nekocode/interaction/connect/feishu"
	"nekocode/interaction/connect/qqbot"
	"nekocode/interaction/connect/telegram"
	controlruntime "nekocode/runtime"
)

var _ controlruntime.Backend = (*bot.Bot)(nil)

// New constructs the standard application runtime.
func New() (*controlruntime.Manager, error) {
	rt := controlruntime.New(bot.New())
	if err := rt.EnableDefaultEventRecording(); err != nil {
		rt.Close()
		return nil, fmt.Errorf("standard runtime: %w", err)
	}
	rt.RegisterConnector("telegram", func(runtime controlruntime.ConnectorRuntime) controlruntime.Connector {
		return telegram.New(runtime)
	})
	rt.RegisterConnector("feishu", func(runtime controlruntime.ConnectorRuntime) controlruntime.Connector {
		return feishu.New(runtime)
	})
	rt.RegisterConnector("qqbot", func(runtime controlruntime.ConnectorRuntime) controlruntime.Connector {
		return qqbot.New(runtime)
	})
	return rt, nil
}
