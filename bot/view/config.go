package view

import "nekocode/bot/config"

func NewConfigView(cfg config.Config) ConfigView {
	return ConfigView{
		Path:           config.Path(),
		Exists:         config.Exists(),
		Active:         cfg.Active,
		ContextWindow:  cfg.ContextWindow,
		FlashModel:     cfg.FlashModel,
		Models:         modelConfigsToView(cfg.Models),
		ImageGenModels: imageGenConfigsToView(cfg.ImageGenModels),
		MCPServers:     mcpServerConfigsToView(cfg.MCPServers),
		Permissions:    permissionsConfigToView(cfg.Permissions),
		Workspaces:     workspaceConfigsToView(cfg.Workspaces),
	}
}

func ToConfig(v ConfigView) config.Config {
	return config.Config{
		Active:         v.Active,
		ContextWindow:  v.ContextWindow,
		FlashModel:     v.FlashModel,
		Models:         modelConfigsFromView(v.Models),
		ImageGenModels: imageGenConfigsFromView(v.ImageGenModels),
		MCPServers:     mcpServerConfigsFromView(v.MCPServers),
		Permissions:    permissionsConfigFromView(v.Permissions),
		Workspaces:     workspaceConfigsFromView(v.Workspaces),
	}
}

func modelConfigsToView(in []config.ModelConfig) []ModelConfig {
	if in == nil {
		return nil
	}
	out := make([]ModelConfig, 0, len(in))
	for _, m := range in {
		out = append(out, ModelConfig{
			Name:     m.Name,
			Provider: m.Provider,
			APIKey:   m.APIKey,
			Model:    m.Model,
			BaseURL:  m.BaseURL,
			Protocol: m.Protocol,
		})
	}
	return out
}

func modelConfigsFromView(in []ModelConfig) []config.ModelConfig {
	if in == nil {
		return nil
	}
	out := make([]config.ModelConfig, 0, len(in))
	for _, m := range in {
		out = append(out, config.ModelConfig{
			Name:     m.Name,
			Provider: m.Provider,
			APIKey:   m.APIKey,
			Model:    m.Model,
			BaseURL:  m.BaseURL,
			Protocol: m.Protocol,
		})
	}
	return out
}

func imageGenConfigsToView(in []config.ImageGenConfig) []ImageGenConfig {
	if in == nil {
		return nil
	}
	out := make([]ImageGenConfig, 0, len(in))
	for _, m := range in {
		out = append(out, ImageGenConfig{
			Name:      m.Name,
			Provider:  m.Provider,
			APIKey:    m.APIKey,
			SecretKey: m.SecretKey,
			BaseURL:   m.BaseURL,
			Model:     m.Model,
		})
	}
	return out
}

func imageGenConfigsFromView(in []ImageGenConfig) []config.ImageGenConfig {
	if in == nil {
		return nil
	}
	out := make([]config.ImageGenConfig, 0, len(in))
	for _, m := range in {
		out = append(out, config.ImageGenConfig{
			Name:      m.Name,
			Provider:  m.Provider,
			APIKey:    m.APIKey,
			SecretKey: m.SecretKey,
			BaseURL:   m.BaseURL,
			Model:     m.Model,
		})
	}
	return out
}

func mcpServerConfigsToView(in map[string]config.MCPServerConfig) map[string]MCPServerConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]MCPServerConfig, len(in))
	for name, srv := range in {
		out[name] = MCPServerConfig{
			Command: srv.Command,
			Args:    append([]string(nil), srv.Args...),
			Env:     stringMap(srv.Env),
			Enabled: srv.Enabled,
		}
	}
	return out
}

func mcpServerConfigsFromView(in map[string]MCPServerConfig) map[string]config.MCPServerConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]config.MCPServerConfig, len(in))
	for name, srv := range in {
		out[name] = config.MCPServerConfig{
			Command: srv.Command,
			Args:    append([]string(nil), srv.Args...),
			Env:     stringMap(srv.Env),
			Enabled: srv.Enabled,
		}
	}
	return out
}

func permissionsConfigToView(in *config.PermissionsConfig) *PermissionsConfig {
	if in == nil {
		return nil
	}
	return &PermissionsConfig{
		Allow:   append([]string(nil), in.Allow...),
		Ask:     append([]string(nil), in.Ask...),
		Deny:    append([]string(nil), in.Deny...),
		Sandbox: sandboxConfigsToView(in.Sandbox),
	}
}

func permissionsConfigFromView(in *PermissionsConfig) *config.PermissionsConfig {
	if in == nil {
		return nil
	}
	return &config.PermissionsConfig{
		Allow:   append([]string(nil), in.Allow...),
		Ask:     append([]string(nil), in.Ask...),
		Deny:    append([]string(nil), in.Deny...),
		Sandbox: sandboxConfigsFromView(in.Sandbox),
	}
}

func sandboxConfigsToView(in map[string]config.SandboxConfig) map[string]SandboxConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]SandboxConfig, len(in))
	for name, sandbox := range in {
		out[name] = SandboxConfig{
			SandboxMode:   sandbox.SandboxMode,
			Network:       sandbox.Network,
			WritableRoots: append([]string(nil), sandbox.WritableRoots...),
		}
	}
	return out
}

func sandboxConfigsFromView(in map[string]SandboxConfig) map[string]config.SandboxConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]config.SandboxConfig, len(in))
	for name, sandbox := range in {
		out[name] = config.SandboxConfig{
			SandboxMode:   sandbox.SandboxMode,
			Network:       sandbox.Network,
			WritableRoots: append([]string(nil), sandbox.WritableRoots...),
		}
	}
	return out
}

func workspaceConfigsToView(in []config.WorkspaceConfig) []WorkspaceConfig {
	if in == nil {
		return nil
	}
	out := make([]WorkspaceConfig, 0, len(in))
	for _, w := range in {
		out = append(out, WorkspaceConfig{Path: w.Path, Access: w.Access})
	}
	return out
}

func workspaceConfigsFromView(in []WorkspaceConfig) []config.WorkspaceConfig {
	if in == nil {
		return nil
	}
	out := make([]config.WorkspaceConfig, 0, len(in))
	for _, w := range in {
		out = append(out, config.WorkspaceConfig{Path: w.Path, Access: w.Access})
	}
	return out
}

func stringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
