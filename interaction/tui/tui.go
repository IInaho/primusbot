// tui.go — package tui 入口。
package tui

import (
	"fmt"

	"nekocode/interaction/tui/components/message"
	"nekocode/runtime/standard"

	tea "charm.land/bubbletea/v2"
)

func Run() error {
	message.Warmup()
	rt, err := standard.New()
	if err != nil {
		return fmt.Errorf("initialize runtime: %w", err)
	}
	defer rt.Close()
	p := tea.NewProgram(NewModel(rt))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
