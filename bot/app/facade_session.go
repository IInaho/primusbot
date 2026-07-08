package app

import (
	"fmt"
	"os"

	"nekocode/bot/command"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/session"
	"nekocode/common"
	"nekocode/common/ui"
)

type sessionFacade struct {
	mgr     *session.Manager
	resumed bool
}

type sessionDeps struct {
	CWD             string
	CtxMgr          *ctxmgr.Manager
	TokenUsage      func() (int, int)
	AddTokens       func(prompt, completion int)
	LoadedSkills    func() map[string]bool
	MarkSkillLoaded func(name string)
	LedgerSnapshot  func() ledger.Snapshot
	RestoreLedger   func(ledger.Snapshot)
}

func newSessionFacade(d sessionDeps) *sessionFacade {
	s := &sessionFacade{}
	s.mgr = session.NewManager(session.ManagerOptions{
		CWD:     d.CWD,
		Context: d.CtxMgr,
		TokenUsage: func() (int, int) {
			if d.TokenUsage == nil {
				return 0, 0
			}
			return d.TokenUsage()
		},
		AddTokens: func(prompt, completion int) {
			if d.AddTokens != nil {
				d.AddTokens(prompt, completion)
			}
		},
		LoadedSkills: func() map[string]bool {
			if d.LoadedSkills == nil {
				return nil
			}
			return d.LoadedSkills()
		},
		MarkSkillLoaded: func(name string) {
			if d.MarkSkillLoaded != nil {
				d.MarkSkillLoaded(name)
			}
		},
		LedgerSnapshot: func() ledger.Snapshot {
			if d.LedgerSnapshot == nil {
				return ledger.Snapshot{}
			}
			return d.LedgerSnapshot()
		},
		RestoreLedger: func(snap ledger.Snapshot) {
			if d.RestoreLedger != nil {
				d.RestoreLedger(snap)
			}
		},
	})

	if err := s.mgr.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "session: %v — running without session persistence\n", err)
	}
	return s
}

func (s *sessionFacade) RegisterCommands(p *command.Parser) {
	p.Register("sessions", func(cmd *command.Command) (string, bool) {
		if len(cmd.Args) > 0 {
			id := cmd.Args[0]
			sess, err := s.mgr.Resume(id)
			if err != nil {
				return session.ResumeFailed(id, err), true
			}
			s.resumed = true
			return session.ResumeSuccess(id, len(sess.Messages)), true
		}
		return session.FormatSessionList(session.List()), true
	})
	p.Register("export", func(cmd *command.Command) (string, bool) {
		path, msgCount, err := s.mgr.Export(session.DefaultExportPath)
		if err != nil {
			return session.ExportFailed(err), true
		}
		return session.ExportSuccess(path, msgCount), true
	})
}

func (s *sessionFacade) Save() {
	if err := s.mgr.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "session: save error: %v\n", err)
	}
}

func (s *sessionFacade) Resume(id string) error {
	_, err := s.mgr.Resume(id)
	return err
}

func (s *sessionFacade) DrainResumed() bool {
	r := s.resumed
	s.resumed = false
	return r
}

func (s *sessionFacade) CWD() string                { return s.mgr.CWD() }
func (s *sessionFacade) CurrentID() string          { return s.mgr.CurrentID() }
func (s *sessionFacade) Set(sess *session.Snapshot) { s.mgr.Set(sess) }
func (s *sessionFacade) ClearContext()              { s.mgr.ClearContext() }
func (s *sessionFacade) DisplayMessages() []common.DisplayMessage {
	return s.mgr.DisplayMessages()
}

func (b *Bot) CWD() string              { return b.sess.CWD() }
func (b *Bot) CurrentSessionID() string { return b.sess.CurrentID() }

// SetSession loads the session with the given id and makes it current.
func (b *Bot) SetSession(id string) error {
	if err := b.sess.Resume(id); err != nil {
		return err
	}
	b.syncHookSessionID()
	return nil
}

func (b *Bot) ClearContext() { b.sess.ClearContext() }
func (b *Bot) SessionMessages() []common.DisplayMessage {
	return b.sess.DisplayMessages()
}

func (b *Bot) ResumeSession(id string) error {
	err := b.sess.Resume(id)
	if err == nil {
		b.syncHookSessionID()
	}
	return err
}

// ListSessions returns metadata for all persisted sessions.
func (b *Bot) ListSessions() []ui.SessionMeta {
	list := session.List()
	out := make([]ui.SessionMeta, 0, len(list))
	for _, m := range list {
		out = append(out, ui.SessionMeta{
			ID:        m.ID,
			CWD:       m.CWD,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			MsgCount:  m.MsgCount,
		})
	}
	return out
}

// NewSession creates a fresh session and makes it current.
func (b *Bot) NewSession() (ui.SessionMeta, error) {
	sess, err := session.New(b.cwd)
	if err != nil {
		return ui.SessionMeta{}, err
	}
	b.sess.Set(sess)
	b.syncHookSessionID()
	return ui.SessionMeta{
		ID:        sess.ID,
		CWD:       sess.CWD,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
		MsgCount:  0,
	}, nil
}

// DeleteSession removes a persisted session by id. If it was the current
// session, clears context so the next message starts fresh.
func (b *Bot) DeleteSession(id string) error {
	if err := session.Delete(id); err != nil {
		return err
	}
	if b.sess.CurrentID() == id {
		b.sess.ClearContext()
		b.syncHookSessionID()
	}
	return nil
}

func (b *Bot) syncHookSessionID() {
	if b.hookReg != nil && b.sess != nil {
		b.hookReg.SetSessionID(b.sess.CurrentID())
	}
}

func (b *Bot) initSession() {
	b.sess = newSessionFacade(sessionDeps{
		CWD:    b.cwd,
		CtxMgr: b.ctxMgr,
		TokenUsage: func() (int, int) {
			ag := b.getAgent()
			if ag == nil {
				return 0, 0
			}
			return ag.TokenUsage()
		},
		AddTokens: func(prompt, completion int) {
			ag := b.getAgent()
			if ag != nil {
				ag.AddTokens(prompt, completion)
			}
		},
		LoadedSkills: func() map[string]bool {
			if b.ext == nil || b.ext.skills == nil {
				return nil
			}
			return b.ext.skills.LoadedSet()
		},
		MarkSkillLoaded: func(name string) {
			if b.ext != nil && b.ext.skills != nil {
				b.ext.skills.MarkLoaded(name)
			}
		},
		LedgerSnapshot: func() ledger.Snapshot {
			return b.ledgerSnapshot()
		},
		RestoreLedger: func(snap ledger.Snapshot) {
			b.restoreLedger(snap)
		},
	})
}

func (b *Bot) ledgerSnapshot() ledger.Snapshot {
	ag := b.getAgent()
	if ag == nil {
		return ledger.Snapshot{}
	}
	gov := ag.GovernanceManager()
	if gov == nil || gov.Ledger == nil {
		return ledger.Snapshot{}
	}
	return gov.Ledger.Snapshot()
}

func (b *Bot) restoreLedger(snap ledger.Snapshot) {
	ag := b.getAgent()
	if ag == nil {
		return
	}
	gov := ag.GovernanceManager()
	if gov == nil || gov.Ledger == nil {
		return
	}
	gov.Ledger.Restore(snap)
	gov.SyncLedgerToHooks()
}
