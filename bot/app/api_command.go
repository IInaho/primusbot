package app

import (
	"nekocode/bot/view"
)

// api_command.go — Bot API：斜杠命令执行与技能提示（命令结果分类、会话保存联动）。

func (b *Bot) CommandNames() []string { return b.cmd.Names() }

func (b *Bot) ExecuteCommand(input string) (string, view.CmdResult) {
	resp, handled := b.cmd.Execute(input, b.ctxMgr)
	if !handled {
		return "", view.CmdNone
	}

	// Commands like /summarize, /clear, /new modify context state
	// (Archive, CompactBoundary, Messages). Save the session so those
	// changes are persisted — RunAgent already does this after each turn.
	b.sess.Save()

	resumed := b.sess.DrainResumed()
	if resumed {
		b.syncHookSessionID()
	}
	result := commandResult(b.cb.pendingConfirmation(), resumed)
	return resp, result
}

func commandResult(pendingConfirm, sessionResumed bool) view.CmdResult {
	switch {
	case pendingConfirm:
		return view.CmdConfirming
	case sessionResumed:
		return view.CmdSessionResumed
	default:
		return view.CmdHandled
	}
}

func (b *Bot) SkillHint() (string, bool) {
	return b.cmd.DrainSkillHint()
}
