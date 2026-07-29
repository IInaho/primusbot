package app

import (
	"fmt"
	"os"

	"nekocode/bot/command"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/session"
	"nekocode/bot/view"
)

type sessionFacade struct {
	mgr     *session.Manager
	resumed bool
}

func newSessionFacade(mgr *session.Manager) *sessionFacade {
	if err := mgr.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "session: %v — running without session persistence\n", err)
	}
	return &sessionFacade{mgr: mgr}
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

func (s *sessionFacade) SaveIfNotEmpty() error {
	// Manager.Save now removes empty sessions from disk instead of writing
	// them; kept as a thin alias for the interrupted-run call site.
	if err := s.mgr.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "session: save error: %v\n", err)
		return err
	}
	return nil
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
func (s *sessionFacade) DisplayMessages() []view.DisplayMessage {
	if s.mgr.Context() == nil {
		return nil
	}
	snap := s.mgr.Context().Snapshot()
	return view.DisplayMessages(snap.Messages, snap.CompactBoundary)
}

func (b *Bot) CWD() string              { return b.sess.CWD() }
func (b *Bot) CurrentSessionID() string { return b.sess.CurrentID() }

// SetSession loads the session with the given id and makes it current.
func (b *Bot) SetSession(id string) error {
	return b.ResumeSession(id)
}

func (b *Bot) ClearContext() { b.sess.ClearContext() }
func (b *Bot) SessionMessages() []view.DisplayMessage {
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
func (b *Bot) ListSessions() []view.SessionMeta {
	return view.SessionMetas(session.List())
}

// NewSession creates a fresh session and makes it current.
func (b *Bot) NewSession() (view.SessionMeta, error) {
	sess, err := session.New(b.cwd)
	if err != nil {
		return view.SessionMeta{}, err
	}
	b.sess.Set(sess)
	b.syncHookSessionID()
	return view.NewSessionMeta(sess.ID, sess.CWD, sess.CreatedAt, sess.UpdatedAt, 0), nil
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
	mgr := session.NewManager(session.ManagerOptions{
		CWD:     b.cwd,
		Context: b.ctxMgr,
		TokenUsage: func() (int, int) {
			ag := b.getAgent()
			if ag == nil {
				return 0, 0
			}
			return ag.TokenUsage()
		},
		AddTokens: func(prompt, completion int) {
			if ag := b.getAgent(); ag != nil {
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
		LedgerSnapshot: b.ledgerSnapshot,
		RestoreLedger:  b.restoreLedger,
	})
	b.sess = newSessionFacade(mgr)
}

func (b *Bot) ledgerSnapshot() ledger.Snapshot {
	ag := b.getAgent()
	if ag == nil {
		return ledger.Snapshot{}
	}
	gov := ag.Governance()
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
	gov := ag.Governance()
	if gov == nil || gov.Ledger == nil {
		return
	}
	gov.Ledger.Restore(snap)
	gov.SyncLedgerToHooks()
}
