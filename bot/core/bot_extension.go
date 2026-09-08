package core

import (
	"context"
	"fmt"

	"nekocode/bot/extension"
	"nekocode/bot/extension/mcp"
	"nekocode/logger"
)

func (b *Bot) initExtensions() {
	b.ext = extension.New(extension.Config{
		Context: b.ctxMgr, Tools: b.toolbox.Registry,
		Policy: b.policy, ContextWindow: b.cfg.EffectiveContextWindow(),
	})

	b.initConfigMCPServers()
	b.ext.Load()
}

func (b *Bot) initConfigMCPServers() {
	for name, cfg := range b.cfg.MCPServers {
		if !cfg.Enabled {
			continue
		}
		err := b.ext.AddMCPServerBackground(name, mcp.ServerConfig{
			Command: cfg.Command,
			Args:    append([]string(nil), cfg.Args...),
			Env:     cfg.Env,
		})
		if err != nil {
			logger.Log("config mcp %s: %v", name, err)
		}
	}
}

func (b *Bot) SelectSkill(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sk, ok := b.ext.Skill(name)
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	b.cmd.SelectSkill(b.ctxMgr, sk.Context)
	b.ext.MarkSkillLoaded(name)
	return nil
}

func (b *Bot) ClearSelectedSkill() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cmd.ClearSkill(b.ctxMgr)
}

func (b *Bot) Extensions() extension.Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.Snapshot()
}

// ReplaceSessionMCPServers atomically replaces transport-supplied MCP servers.
func (b *Bot) ReplaceSessionMCPServers(ctx context.Context, source string, configs map[string]mcp.ServerConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.ReplaceSessionMCPServers(ctx, source, configs)
}

func (b *Bot) SetPluginEnabled(name string, enabled bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.SetPluginEnabled(name, enabled)
}

func (b *Bot) RefreshExtensions() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ext.Reload()
}
