package core

import (
	"fmt"
	"reflect"

	"nekocode/bot/command"
	"nekocode/bot/config"
	"nekocode/bot/contextmgr"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/prompt"
	"nekocode/protocol"
)

func (b *Bot) environment() prompt.Environment {
	env := prompt.Environment{Cwd: b.cwd}
	if b.toolbox == nil {
		return env
	}
	for _, root := range b.toolbox.Workspace().Snapshot() {
		env.Roots = append(env.Roots, prompt.Root{Path: root.Path, Access: string(root.Access)})
	}
	env.ManagedProcesses = b.toolbox.ManagedProcessSummary()
	return env
}

type Memory struct {
	Path    string
	Content string
}

func (b *Bot) model() config.ModelConfig {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cfg.ActiveModelConfig()
}

func (b *Bot) SwitchModel(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.cfg.SwitchModel(name) {
		return fmt.Errorf("model %q not found. Available: %v", name, b.cfg.AllModelNames())
	}

	oldPrompt, oldCompletion := b.ag.TokenUsage()
	b.initAgent()
	b.ag.AddTokens(oldPrompt, oldCompletion)
	return nil
}

func (b *Bot) Metrics() protocol.Metrics {
	agent := b.getAgent()
	return agent.Metrics()
}

func (b *Bot) Configuration() config.Config {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cfg.Clone()
}

func (b *Bot) ApplyConfiguration(next config.Config) error {
	next = next.Clone()
	if err := config.Validate(&next); err != nil {
		return err
	}

	b.mu.Lock()
	authorityChanged := sandboxAuthorityChanged(b.cfg, next)
	oldPrompt, oldCompletion := 0, 0
	if b.ag != nil {
		oldPrompt, oldCompletion = b.ag.TokenUsage()
	}
	b.mu.Unlock()

	// Revoke processes launched under the old sandbox authority before the new
	// authority is persisted or installed. If termination cannot be confirmed,
	// leave the active and persisted configuration unchanged.
	if authorityChanged && b.sess != nil {
		if err := b.closeSessionRuntime(b.sess.CurrentID()); err != nil {
			return err
		}
	}
	if err := config.Save(next); err != nil {
		return err
	}

	b.mu.Lock()
	b.cfg = &next
	b.mu.Unlock()
	return b.reloadRuntime(oldPrompt, oldCompletion)
}

func sandboxAuthorityChanged(current *config.Config, next config.Config) bool {
	return current == nil ||
		!reflect.DeepEqual(current.Permissions, next.Permissions) ||
		!reflect.DeepEqual(current.Workspaces, next.Workspaces)
}

func (b *Bot) reloadRuntime(oldPrompt, oldCompletion int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.rebuildRuntime(); err != nil {
		return err
	}
	if b.ag != nil {
		b.ag.AddTokens(oldPrompt, oldCompletion)
	}
	return nil
}

func (b *Bot) ContextReport() contextmgr.ContextReport {
	b.mu.Lock()
	defer b.mu.Unlock()

	report := b.ctxMgr.Report()
	descriptors := b.toolbox.Registry.Descriptors()
	report.ToolDefCount = len(descriptors)
	report.ToolDefTokens = command.EstimateToolDefTokens(descriptors)
	return report
}

func (b *Bot) Memory() Memory {
	b.mu.Lock()
	defer b.mu.Unlock()
	return Memory{Path: memory.DefaultPath(), Content: b.ctxMgr.Snapshot().Memory}
}
