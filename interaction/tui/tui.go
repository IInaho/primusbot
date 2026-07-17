// tui.go — package tui 入口。
package tui

import (
	"fmt"

	"nekocode/interaction/connect/telegram"
	"nekocode/interaction/tui/components/message"
	controlruntime "nekocode/runtime"
	"nekocode/runtime/defaultbot"

	tea "charm.land/bubbletea/v2"
)

func Run() {
	message.Warmup()
	rt := defaultbot.NewSessionRuntime()
	rt.RegisterConnector("telegram", func(runtime controlruntime.Runtime) controlruntime.Connector {
		return telegram.New(runtime)
	})
	defer rt.Close()
	p := tea.NewProgram(NewModel(rt))
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
