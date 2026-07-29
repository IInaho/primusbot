package app

import (
	"fmt"
	"os"

	"nekocode/bot/command"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/session"
	"nekocode/bot/view"
)

func (b *Bot) initSession() {
	b.sess = session.NewManager(session.ManagerOptions{
		CWD:     b.cwd,
		Context: b.ctxMgr,
		TokenUsage: func() (int, int) {
			if ag := b.getAgent(); ag != nil {
				return ag.TokenUsage()
			}
			return 0, 0
		},
		AddTokens: func(prompt, completion int) {
			if ag := b.getAgent(); ag != nil {
				ag.AddTokens(prompt, completion)
			}
		},
		LoadedSkills: func() map[string]bool {
			if b.ext == nil {
				return nil
			}
			return b.ext.Snapshot().LoadedSkills
		},
		MarkSkillLoaded: func(name string) {
			if b.ext != nil {
				b.ext.MarkSkillLoaded(name)
			}
		},
		LedgerSnapshot: b.ledgerSnapshot,
		RestoreLedger:  b.restoreLedger,
	})
	if err := b.sess.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "session: %v - running without session persistence\n", err)
	}
}

func (b *Bot) registerSessionCommands(p *command.Parser) {
	p.Register("sessions", func(cmd *command.Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return session.FormatSessionList(session.List()), true
		}
		id := cmd.Args[0]
		snapshot, err := b.sess.Resume(id)
		if err != nil {
			return session.ResumeFailed(id, err), true
		}
		b.sessionResumed = true
		return session.ResumeSuccess(id, len(snapshot.Messages)), true
	})
	p.Register("export", func(*command.Command) (string, bool) {
		path, msgCount, err := b.sess.Export(session.DefaultExportPath)
		if err != nil {
			return session.ExportFailed(err), true
		}
		return session.ExportSuccess(path, msgCount), true
	})
}

func (b *Bot) saveSession() error {
	err := b.sess.Save()
	if err != nil {
		fmt.Fprintf(os.Stderr, "session: save error: %v\n", err)
	}
	return err
}

func (b *Bot) drainSessionResumed() bool {
	resumed := b.sessionResumed
	b.sessionResumed = false
	return resumed
}

func (b *Bot) CWD() string              { return b.sess.CWD() }
func (b *Bot) CurrentSessionID() string { return b.sess.CurrentID() }
func (b *Bot) SetSession(id string) error {
	return b.ResumeSession(id)
}
func (b *Bot) ClearContext() { b.sess.ClearContext() }

func (b *Bot) SessionMessages() []view.DisplayMessage {
	ctx := b.sess.Context()
	if ctx == nil {
		return nil
	}
	snapshot := ctx.Snapshot()
	return view.DisplayMessages(snapshot.Messages, snapshot.CompactBoundary)
}

func (b *Bot) ResumeSession(id string) error {
	_, err := b.sess.Resume(id)
	if err == nil {
		b.syncPolicySessionID()
	}
	return err
}

func (b *Bot) ListSessions() []view.SessionMeta {
	return view.SessionMetas(session.List())
}

func (b *Bot) NewSession() (view.SessionMeta, error) {
	snapshot, err := session.New(b.cwd)
	if err != nil {
		return view.SessionMeta{}, err
	}
	b.sess.Set(snapshot)
	b.syncPolicySessionID()
	return view.NewSessionMeta(snapshot.ID, snapshot.CWD, snapshot.CreatedAt, snapshot.UpdatedAt, 0), nil
}

func (b *Bot) DeleteSession(id string) error {
	if err := session.Delete(id); err != nil {
		return err
	}
	if b.sess.CurrentID() == id {
		b.sess.ClearContext()
		b.syncPolicySessionID()
	}
	return nil
}

func (b *Bot) syncPolicySessionID() {
	if b.policy != nil && b.sess != nil {
		b.policy.SetSessionID(b.sess.CurrentID())
	}
}

func (b *Bot) ledgerSnapshot() ledger.Snapshot {
	ag := b.getAgent()
	if ag == nil || ag.Governance() == nil {
		return ledger.Snapshot{}
	}
	return ag.Governance().Snapshot()
}

func (b *Bot) restoreLedger(snapshot ledger.Snapshot) {
	ag := b.getAgent()
	if ag != nil && ag.Governance() != nil {
		ag.Governance().Restore(snapshot)
	}
}
