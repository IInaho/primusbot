package policy

import (
	"strings"
	"testing"
)

func TestHookStatePersistsAndIsPrivate(t *testing.T) {
	engine := newHookEngine()
	var firstSeen, secondSeen int64
	engine.register(Hook{
		Name:  "first",
		Point: PreTurn,
		On: func(state State) *Result {
			firstSeen = state.Int("count")
			state.SetInt("count", firstSeen+1)
			return nil
		},
	})
	engine.register(Hook{
		Name:  "second",
		Point: PreTurn,
		On: func(state State) *Result {
			secondSeen = state.Int("count")
			return nil
		},
	})

	engine.evaluate(PreTurn, Facts{})
	engine.evaluate(PreTurn, Facts{})
	if firstSeen != 1 {
		t.Fatalf("first hook count = %d, want 1 on second evaluation", firstSeen)
	}
	if secondSeen != 0 {
		t.Fatalf("second hook observed first hook state: %d", secondSeen)
	}
}

func TestHookReceivesImmutableFactsValue(t *testing.T) {
	engine := newHookEngine()
	facts := Facts{
		Turn: TurnFacts{Input: "task"},
		Tool: ToolFacts{Args: map[string]any{"path": "main.go"}},
	}
	engine.register(Hook{
		Name:  "capture",
		Point: PreTurn,
		On: func(state State) *Result {
			got := state.Facts()
			got.Turn.Input = "changed"
			got.Tool.Args["path"] = "changed.go"
			return nil
		},
	})

	engine.evaluate(PreTurn, facts)
	if facts.Turn.Input != "task" {
		t.Fatalf("source facts mutated: %+v", facts)
	}
	if facts.Tool.Args["path"] != "main.go" {
		t.Fatalf("source args mutated: %+v", facts.Tool.Args)
	}
}

func TestHookAuditAndStats(t *testing.T) {
	engine := newHookEngine()
	engine.setSessionID("session-1")
	engine.register(Hook{
		Name:  "block",
		Point: PreToolUse,
		On: func(State) *Result {
			return &Result{BlockTool: &BlockTool{Tool: "edit", Reason: "read first"}}
		},
	})

	engine.evaluate(PreToolUse, Facts{Tool: ToolFacts{Name: "edit"}})
	stats := engine.summary()
	if !strings.Contains(stats, "block_tool=block") || !strings.Contains(stats, "block@pre_tool_use:block_tool") {
		t.Fatalf("stats = %q", stats)
	}
	if second := engine.summary(); second != stats {
		t.Fatalf("second summary = %q, want stable %q", second, stats)
	}
}

func TestUnregisterWhereRemovesHookAndState(t *testing.T) {
	engine := newHookEngine()
	engine.register(Hook{
		Name:  "drop",
		Point: PreTurn,
		On: func(state State) *Result {
			state.SetInt("count", 1)
			return nil
		},
	})
	engine.evaluate(PreTurn, Facts{})
	engine.unregisterPrefix("drop")

	if len(engine.list()) != 0 {
		t.Fatalf("hooks = %+v, want empty", engine.list())
	}
	if _, ok := engine.state["drop"]; ok {
		t.Fatal("unregistered hook state was retained")
	}
}

func TestRegisterReplacesHookWithSameName(t *testing.T) {
	engine := newHookEngine()
	engine.register(Hook{Name: "guard", Point: PreTurn, On: func(State) *Result { return nil }})
	engine.register(Hook{Name: "guard", Point: PostTurn, On: func(State) *Result { return nil }})

	hooks := engine.list()
	if len(hooks) != 1 || hooks[0].Point != PostTurn {
		t.Fatalf("hooks = %+v, want replacement", hooks)
	}
}
