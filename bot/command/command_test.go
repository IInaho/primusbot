package command

import (
	"context"
	"testing"

	"nekocode/bot/config"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/skill"
	"nekocode/bot/tools"
)

type emptySkills struct{}

func (emptySkills) SkillCommands() []skill.Command     { return nil }
func (emptySkills) Skill(string) (skill.Command, bool) { return skill.Command{}, false }
func (emptySkills) MarkSkillLoaded(string)             {}
func (emptySkills) ClearLoadedSkills()                 {}

func TestNewBuildsCommandHandler(t *testing.T) {
	handler := New(Deps{
		CtxMgr:       ctxmgr.New(ctxmgr.Config{}),
		Ag:           func() PlanModeController { return nil },
		Skills:       emptySkills{},
		ToolRegistry: tools.New(),
		GetConfigFn:  func() config.ModelConfig { return config.ModelConfig{} },
		SwitchModel:  func(string) error { return nil },
	})

	if len(handler.Names()) == 0 {
		t.Fatal("New registered no commands")
	}
	if response, handled := handler.Execute(context.Background(), "/help", ctxmgr.New(ctxmgr.Config{})); !handled || response == "" {
		t.Fatalf("help result = %q, handled = %v", response, handled)
	}
}
