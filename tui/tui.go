// tui.go — package tui 入口。
package tui

import (
	"fmt"

	"nekocode/bot"
	"nekocode/tui/components/message"

	tea "charm.land/bubbletea/v2"
)

func Run() {
	message.Warmup()
	b := bot.New()
	defer b.Close()
	p := tea.NewProgram(NewModel(b))
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
