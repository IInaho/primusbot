package viewmodel

import (
	"nekocode/bot/config"
	controlruntime "nekocode/runtime"
)

func modelConfigsToView(in []config.ModelConfig) []controlruntime.ModelConfig {
	if in == nil {
		return nil
	}
	out := make([]controlruntime.ModelConfig, 0, len(in))
	for _, m := range in {
		out = append(out, controlruntime.ModelConfig{
			Name:          m.Name,
			Provider:      m.Provider,
			APIKey:        m.APIKey,
			Model:         m.Model,
			BaseURL:       m.BaseURL,
			Protocol:      m.Protocol,
			ContextWindow: m.ContextWindow,
		})
	}
	return out
}

func modelConfigsFromView(in []controlruntime.ModelConfig) []config.ModelConfig {
	if in == nil {
		return nil
	}
	out := make([]config.ModelConfig, 0, len(in))
	for _, m := range in {
		out = append(out, config.ModelConfig{
			Name:          m.Name,
			Provider:      m.Provider,
			APIKey:        m.APIKey,
			Model:         m.Model,
			BaseURL:       m.BaseURL,
			Protocol:      m.Protocol,
			ContextWindow: m.ContextWindow,
		})
	}
	return out
}

func imageGenConfigsToView(in []config.ImageGenConfig) []controlruntime.ImageGenConfig {
	if in == nil {
		return nil
	}
	out := make([]controlruntime.ImageGenConfig, 0, len(in))
	for _, m := range in {
		out = append(out, controlruntime.ImageGenConfig{
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

func imageGenConfigsFromView(in []controlruntime.ImageGenConfig) []config.ImageGenConfig {
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

func mcpServerConfigsToView(in map[string]config.MCPServerConfig) map[string]controlruntime.MCPServerConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]controlruntime.MCPServerConfig, len(in))
	for name, srv := range in {
		out[name] = controlruntime.MCPServerConfig{
			Command: srv.Command,
			Args:    append([]string(nil), srv.Args...),
			Env:     stringMap(srv.Env),
			Enabled: srv.Enabled,
		}
	}
	return out
}

func mcpServerConfigsFromView(in map[string]controlruntime.MCPServerConfig) map[string]config.MCPServerConfig {
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

func permissionsConfigToView(in *config.PermissionsConfig) *controlruntime.PermissionsConfig {
	if in == nil {
		return nil
	}
	return &controlruntime.PermissionsConfig{
		Allow:   append([]string(nil), in.Allow...),
		Ask:     append([]string(nil), in.Ask...),
		Deny:    append([]string(nil), in.Deny...),
		Sandbox: sandboxConfigsToView(in.Sandbox),
	}
}

func permissionsConfigFromView(in *controlruntime.PermissionsConfig) *config.PermissionsConfig {
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

func sandboxConfigsToView(in map[string]config.SandboxConfig) map[string]controlruntime.SandboxConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]controlruntime.SandboxConfig, len(in))
	for name, sandbox := range in {
		out[name] = controlruntime.SandboxConfig{
			SandboxMode:   sandbox.SandboxMode,
			Network:       sandbox.Network,
			WritableRoots: append([]string(nil), sandbox.WritableRoots...),
		}
	}
	return out
}

func sandboxConfigsFromView(in map[string]controlruntime.SandboxConfig) map[string]config.SandboxConfig {
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

func workspaceConfigsToView(in []config.WorkspaceConfig) []controlruntime.WorkspaceConfig {
	if in == nil {
		return nil
	}
	out := make([]controlruntime.WorkspaceConfig, 0, len(in))
	for _, w := range in {
		out = append(out, controlruntime.WorkspaceConfig{Path: w.Path, Access: w.Access})
	}
	return out
}

func workspaceConfigsFromView(in []controlruntime.WorkspaceConfig) []config.WorkspaceConfig {
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
