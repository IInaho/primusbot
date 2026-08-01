package bot

import (
	"context"
	"fmt"
	"strings"

	"nekocode/bot/command"
	"nekocode/bot/contextmgr"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/session"
)

func (b *Bot) initSession() {
	b.sess = session.New(b.cwd)
}

func (b *Bot) registerSessionCommands(p *command.Parser) {
	p.Register("sessions", func(ctx context.Context, cmd *command.Command) (string, bool) {
		if err := ctx.Err(); err != nil {
			return "Session command cancelled: " + err.Error(), true
		}
		if len(cmd.Args) == 0 {
			return formatSessionList(b.sess.List()), true
		}
		id := cmd.Args[0]
		snapshot, err := b.resumeSession(id)
		if err != nil {
			return fmt.Sprintf("Failed to resume session %s: %v", id, err), true
		}
		b.syncPolicySessionID()
		return fmt.Sprintf("Resumed session %s (%d messages restored).", id, len(snapshot.Messages)), true
	})
	p.Register("export", func(ctx context.Context, _ *command.Command) (string, bool) {
		if err := ctx.Err(); err != nil {
			return "Export cancelled: " + err.Error(), true
		}
		messages := b.ctxMgr.Build()
		path, err := session.ExportMessages(messages, session.DefaultExportPath)
		if err != nil {
			return fmt.Sprintf("Failed to %v", err), true
		}
		return fmt.Sprintf("Context exported to %s (%d messages)", path, len(messages)), true
	})
}

func (b *Bot) saveSession() error {
	snapshot := b.sess.Current()
	if snapshot == nil {
		snapshot = b.sess.StartNew()
	}
	promptTokens, completionTokens := 0, 0
	if ag := b.getAgent(); ag != nil {
		promptTokens, completionTokens = ag.TokenUsage()
	}
	var loadedSkills map[string]bool
	if b.ext != nil {
		loadedSkills = b.ext.Snapshot().LoadedSkills
	}
	snapshot.CaptureContext(b.ctxMgr.Snapshot(), promptTokens, completionTokens, loadedSkills)
	snapshot.Ledger = b.ledgerSnapshot()
	err := b.sess.Save(snapshot)
	b.syncPolicySessionID()
	return err
}

func (b *Bot) CurrentSessionID() string { return b.sess.CurrentID() }

func (b *Bot) Conversation() contextmgr.ManagerSnapshot {
	if b.ctxMgr == nil {
		return contextmgr.ManagerSnapshot{}
	}
	return b.ctxMgr.Snapshot()
}

func (b *Bot) ResumeSession(id string) error {
	_, err := b.resumeSession(id)
	return err
}

func (b *Bot) resumeSession(id string) (*session.Snapshot, error) {
	oldID := b.sess.CurrentID()
	snapshot, err := b.sess.Load(id)
	if err != nil {
		return nil, err
	}
	if oldID != id {
		if err := b.closeSessionRuntime(oldID); err != nil {
			return nil, err
		}
	}
	if err := b.sess.Activate(snapshot); err != nil {
		return nil, err
	}
	b.ctxMgr.Restore(snapshot.ContextSnapshot())
	// Session files may contain a prompt from an older NekoCode version. Keep
	// conversation state, but always pair it with the current stable rules;
	// volatile environment data is injected separately on every Build.
	if b.promptBuilder != nil {
		b.ctxMgr.SetSystemPrompt(b.promptBuilder.BuildStatic())
	}
	if ag := b.getAgent(); ag != nil {
		ag.AddTokens(snapshot.PromptTokens, snapshot.CompletionTokens)
	}
	if b.ext != nil {
		b.ext.ClearLoadedSkills()
		for _, name := range snapshot.LoadedSkills {
			b.ext.MarkSkillLoaded(name)
		}
	}
	b.restoreLedger(snapshot.Ledger)
	if b.cmd != nil {
		b.cmd.ResetSkill()
	}
	b.syncPolicySessionID()
	return snapshot, nil
}

func (b *Bot) ListSessions() []session.Meta {
	return b.sess.List()
}

func (b *Bot) NewSession() (*session.Snapshot, error) {
	_, err := b.resetConversation(false)
	if err != nil {
		return nil, err
	}
	return b.sess.Current(), nil
}

func (b *Bot) resetConversation(keepSummary bool) (string, error) {
	result := "Conversation history cleared."
	var previousContext contextmgr.ManagerSnapshot
	if keepSummary {
		previousContext = b.ctxMgr.Snapshot()
		count, oldTokens, _ := b.ctxMgr.Stats()
		if count <= 2 {
			result = "New session started."
		} else {
			if b.ctxMgr.NeedsSummarization() {
				if err := b.ctxMgr.Summarize(); err != nil {
					return "", err
				}
			}
			_, newTokens, hasSummary := b.ctxMgr.Stats()
			summary := "no summary"
			if hasSummary {
				summary = "with summary"
			}
			result = fmt.Sprintf("%d messages, ~%d tokens → %s (~%d tokens)", count, oldTokens, summary, newTokens)
		}
	}
	if err := b.closeSessionRuntime(b.sess.CurrentID()); err != nil {
		if keepSummary {
			b.ctxMgr.Restore(previousContext)
		}
		return "", err
	}
	if keepSummary {
		b.ctxMgr.FreshStart()
	} else {
		b.ctxMgr.Clear()
	}
	b.sess.StartNew()
	if b.ext != nil {
		b.ext.ClearLoadedSkills()
	}
	if b.cmd != nil {
		b.cmd.ResetSkill()
	}
	if b.policy != nil {
		b.policy.Restore(ledger.Snapshot{})
	}
	b.syncPolicySessionID()
	return result, nil
}

func (b *Bot) ensureSessionIdentity() {
	if b.sess == nil {
		return
	}
	if b.sess.CurrentID() == "" {
		b.sess.StartNew()
	}
	b.syncPolicySessionID()
}

func (b *Bot) DeleteSession(id string) error {
	if err := b.closeSessionRuntime(id); err != nil {
		return err
	}
	if err := b.sess.Delete(id); err != nil {
		return err
	}
	if b.sess.CurrentID() == id {
		b.ctxMgr.Clear()
		b.sess.ClearCurrent()
		if b.ext != nil {
			b.ext.ClearLoadedSkills()
		}
		if b.cmd != nil {
			b.cmd.ResetSkill()
		}
		if b.policy != nil {
			b.policy.Restore(ledger.Snapshot{})
		}
		b.syncPolicySessionID()
	}
	return nil
}

func formatSessionList(sessions []session.Meta) string {
	if len(sessions) == 0 {
		return "No saved sessions."
	}
	var out strings.Builder
	out.WriteString("Saved sessions:\n")
	for _, item := range sessions {
		fmt.Fprintf(&out, "  %s  %s  %d msgs  %s\n", item.ID, item.Age(), item.MsgCount, item.CWD)
	}
	out.WriteString("\n/sessions <id> to resume")
	return out.String()
}

func (b *Bot) syncPolicySessionID() {
	if b.sess == nil {
		return
	}
	id := b.sess.CurrentID()
	if b.policy != nil {
		b.policy.SetSessionID(id)
	}
	if b.toolbox != nil {
		b.toolbox.SetSessionID(id)
	}
}

func (b *Bot) closeSessionRuntime(id string) error {
	if id == "" || b.toolbox == nil {
		return nil
	}
	return b.toolbox.CloseSession(id)
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
