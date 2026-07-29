// tui.go — package tui 入口。
package tui

import (
	"fmt"

	"nekocode/interaction/tui/components/message"
	"nekocode/runtime/defaultbot"

	tea "charm.land/bubbletea/v2"
)

func Run() {
	message.Warmup()
	rt := defaultbot.NewSessionRuntimeWithConnectors()
	defer rt.Close()
	p := tea.NewProgram(NewModel(rt))
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
