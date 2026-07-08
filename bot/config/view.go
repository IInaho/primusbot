package config

import (
	"nekocode/common/ui"
)

// View is the UI-facing config view. It now lives in common/ui.
type View = ui.ConfigView

func NewView(cfg Config) View {
	return View{
		Path:           Path(),
		Exists:         Exists(),
		Active:         cfg.Active,
		ContextWindow:  cfg.ContextWindow,
		FlashModel:     cfg.FlashModel,
		Models:         cfg.Models,
		ImageGenModels: cfg.ImageGenModels,
		MCPServers:     cfg.MCPServers,
		Permissions:    cfg.Permissions,
		Workspaces:     cfg.Workspaces,
	}
}

// ToConfig converts a View back to a Config.
func ToConfig(v View) Config {
	return Config{
		Active:         v.Active,
		ContextWindow:  v.ContextWindow,
		FlashModel:     v.FlashModel,
		Models:         v.Models,
		ImageGenModels: v.ImageGenModels,
		MCPServers:     v.MCPServers,
		Permissions:    v.Permissions,
		Workspaces:     v.Workspaces,
	}
}
