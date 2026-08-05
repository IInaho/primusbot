package command

import (
	"context"
	"testing"

	"nekocode/bot/config"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/tool"
)

func TestNewBuildsCommandHandler(t *testing.T) {
	handler := New(Deps{
		CtxMgr:       ctxmgr.New(ctxmgr.Config{}),
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
		CtxMgr:       manager,
		ToolRegistry: tools.New(),
		GetConfigFn:  func() config.ModelConfig { return config.ModelConfig{} },
		SwitchModel:  func(string) error { return nil },
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

func TestRewindCommandDelegatesOptionalTurn(t *testing.T) {
	manager := ctxmgr.New(ctxmgr.Config{})
	var got string
	handler := New(Deps{
		CtxMgr: manager, ToolRegistry: tools.New(),
		GetConfigFn: func() config.ModelConfig { return config.ModelConfig{} },
		Rewind: func(turn string) (string, error) {
			got = turn
			return "rewound", nil
		},
	})
	result, handled := handler.Execute(context.Background(), "/rewind 2", manager)
	if !handled || result != "rewound" || got != "2" {
		t.Fatalf("rewind result=%q handled=%v turn=%q", result, handled, got)
	}
}

func TestRewindListUsesSameCheckpointCommand(t *testing.T) {
	manager := ctxmgr.New(ctxmgr.Config{})
	var got string
	handler := New(Deps{
		CtxMgr: manager, ToolRegistry: tools.New(),
		GetConfigFn: func() config.ModelConfig { return config.ModelConfig{} },
		Rewind: func(turn string) (string, error) {
			got = turn
			return "history", nil
		},
	})
	result, handled := handler.Execute(context.Background(), "/rewind list", manager)
	if !handled || result != "history" || got != "list" {
		t.Fatalf("rewind list result=%q handled=%v arg=%q", result, handled, got)
	}
}

func TestRegisterSkillsReplacesDynamicCommands(t *testing.T) {
	manager := ctxmgr.New(ctxmgr.Config{})
	handler := New(Deps{
		CtxMgr: manager, ToolRegistry: tools.New(),
		GetConfigFn: func() config.ModelConfig { return config.ModelConfig{} },
	})
	loaded := false
	handler.RegisterSkills([]SkillRegistration{{
		Name: "review", Load: func() (string, bool) { return "review context", true }, MarkLoaded: func() { loaded = true },
	}})
	if _, handled := handler.Execute(context.Background(), "$review inspect", manager); handled {
		t.Fatal("skill with arguments should continue into the agent")
	}
	if !loaded {
		t.Fatal("skill loaded callback was not invoked")
	}

	handler.RegisterSkills(nil)
	if result, handled := handler.Execute(context.Background(), "$review", manager); !handled || result == "" {
		t.Fatal("replaced skill command remained registered")
	}
}
