package command

import (
	"testing"

	"nekocode/bot/tools/runtime/core"
)

func TestEstimateToolDefTokens(t *testing.T) {
	descs := []core.Descriptor{
		{Name: "read", Description: "read files", Parameters: []core.Parameter{
			{Name: "path", Type: "string", Description: "file path"},
		}},
	}
	n := EstimateToolDefTokens(descs)
	if n <= 0 {
		t.Errorf("expected positive token count, got %d", n)
	}
}

func TestSkillState(t *testing.T) {
	st := &skillState{MsgStart: -1}
	if clearSkillContext(nil, st); st.MsgStart != -1 {
		t.Error("should be no-op when MsgStart is -1")
	}
}
