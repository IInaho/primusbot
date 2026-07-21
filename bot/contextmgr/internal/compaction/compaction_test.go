package compaction

import (
	"testing"

	compress "nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
)

func TestAutoCompactIfNeeded_NoStrategy(t *testing.T) {
	c := &Controller{State: &state.State{
		Ctx:           content.New("test prompt"),
		Tracker:       &token.Tracker{},
		ContextWindow: 10000,
	}}
	level, err := c.AutoCompactIfNeeded()
	if err != nil {
		t.Errorf("AutoCompactIfNeeded error: %v", err)
	}
	if level != compress.LevelNormal {
		t.Errorf("level = %s, want %s", level, compress.LevelNormal)
	}
}
