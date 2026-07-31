// Package standard assembles NekoCode's standard runtime with the default bot,
// event recording and bundled connectors.
package standard

import (
	"errors"
	"fmt"

	"nekocode/bot"
	"nekocode/interaction/connect/feishu"
	"nekocode/interaction/connect/qqbot"
	"nekocode/interaction/connect/telegram"
	controlruntime "nekocode/runtime"
)

// New constructs the standard application runtime.
func New() (*controlruntime.Manager, error) {
	standardBot, err := bot.New()
	if err != nil {
		return nil, fmt.Errorf("standard runtime: %w", err)
	}
	rt := FromBot(standardBot)
	if err := rt.EnableDefaultEventRecording(); err != nil {
		return nil, errors.Join(fmt.Errorf("standard runtime: %w", err), rt.Close())
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

// FromBot creates a runtime around an existing standard bot without enabling
// event recording or registering bundled connectors.
func FromBot(core *bot.Bot) *controlruntime.Manager {
	if core == nil {
		panic("runtime/standard: nil bot")
	}
	return controlruntime.New(adapt(core))
}
