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

func TestConversationCommandsDelegateReset(t *testing.T) {
	manager := ctxmgr.New(ctxmgr.Config{})
	var calls []bool
	handler := New(Deps{
		CtxMgr: manager, Ag: func() PlanModeController { return nil },
		Skills: emptySkills{}, ToolRegistry: tools.New(),
		GetConfigFn: func() config.ModelConfig { return config.ModelConfig{} },
		SwitchModel: func(string) error { return nil },
		ResetConversation: func(keepSummary bool) (string, error) {
			calls = append(calls, keepSummary)
			return "reset", nil
		},
	})
	for _, input := range []string{"/clear", "/new"} {
		if result, handled := handler.Execute(context.Background(), input, manager); !handled || result != "reset" {
			t.Fatalf("%s result=%q handled=%v", input, result, handled)
		}
	}
	if len(calls) != 2 || calls[0] || !calls[1] {
		t.Fatalf("reset modes = %v, want [false true]", calls)
	}
}
