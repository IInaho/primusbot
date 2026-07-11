package view

import "nekocode/bot/config"

func NewConfigView(cfg config.Config) ConfigView {
	return ConfigView{
		Path:           config.Path(),
		Exists:         config.Exists(),
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

func ToConfig(v ConfigView) config.Config {
	return config.Config{
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
