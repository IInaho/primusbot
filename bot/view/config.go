package view

import (
	"nekocode/bot/config"
	commonview "nekocode/common/view"
)

func modelConfigsToView(in []config.ModelConfig) []commonview.ModelConfig {
	if in == nil {
		return nil
	}
	out := make([]commonview.ModelConfig, 0, len(in))
	for _, m := range in {
		out = append(out, commonview.ModelConfig{
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

func modelConfigsFromView(in []commonview.ModelConfig) []config.ModelConfig {
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

func imageGenConfigsToView(in []config.ImageGenConfig) []commonview.ImageGenConfig {
	if in == nil {
		return nil
	}
	out := make([]commonview.ImageGenConfig, 0, len(in))
	for _, m := range in {
		out = append(out, commonview.ImageGenConfig{
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

func imageGenConfigsFromView(in []commonview.ImageGenConfig) []config.ImageGenConfig {
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

func mcpServerConfigsToView(in map[string]config.MCPServerConfig) map[string]commonview.MCPServerConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]commonview.MCPServerConfig, len(in))
	for name, srv := range in {
		out[name] = commonview.MCPServerConfig{
			Command: srv.Command,
			Args:    append([]string(nil), srv.Args...),
			Env:     stringMap(srv.Env),
			Enabled: srv.Enabled,
		}
	}
	return out
}

func mcpServerConfigsFromView(in map[string]commonview.MCPServerConfig) map[string]config.MCPServerConfig {
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

func permissionsConfigToView(in *config.PermissionsConfig) *commonview.PermissionsConfig {
	if in == nil {
		return nil
	}
	return &commonview.PermissionsConfig{
		Allow:   append([]string(nil), in.Allow...),
		Ask:     append([]string(nil), in.Ask...),
		Deny:    append([]string(nil), in.Deny...),
		Sandbox: sandboxConfigsToView(in.Sandbox),
	}
}

func permissionsConfigFromView(in *commonview.PermissionsConfig) *config.PermissionsConfig {
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

func sandboxConfigsToView(in map[string]config.SandboxConfig) map[string]commonview.SandboxConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]commonview.SandboxConfig, len(in))
	for name, sandbox := range in {
		out[name] = commonview.SandboxConfig{
			SandboxMode:   sandbox.SandboxMode,
			Network:       sandbox.Network,
			WritableRoots: append([]string(nil), sandbox.WritableRoots...),
		}
	}
	return out
}

func sandboxConfigsFromView(in map[string]commonview.SandboxConfig) map[string]config.SandboxConfig {
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

func workspaceConfigsToView(in []config.WorkspaceConfig) []commonview.WorkspaceConfig {
	if in == nil {
		return nil
	}
	out := make([]commonview.WorkspaceConfig, 0, len(in))
	for _, w := range in {
		out = append(out, commonview.WorkspaceConfig{Path: w.Path, Access: w.Access})
	}
	return out
}

func workspaceConfigsFromView(in []commonview.WorkspaceConfig) []config.WorkspaceConfig {
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
