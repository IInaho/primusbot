package runtime

import (
	"context"
	"strings"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/hooks"
	"nekocode/bot/hooks/builtin"
	aggov "nekocode/bot/policy"
	"nekocode/bot/provider/types"
	"nekocode/bot/tools"
)

func newTestAgent() *Agent {
	ctxMgr := ctxmgr.NewSub("test", 128000, nil)
	reg := tools.NewRegistry()
	a := New(context.Background(), ctxMgr, nil, reg)
	a.deps.gov = aggov.NewManager(hooks.NewRegistry())
	builtin.Register(a.deps.gov.HookReg)
	return a
}

func messagesContain(msgs []types.Message, substr string) bool {
	for _, msg := range msgs {
		if strings.Contains(msg.Content, substr) {
			return true
		}
	}
	return false
}
