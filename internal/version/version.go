// Package version 提供编译期注入的版本号，供 TUI 等前端展示。
//
// 版本号由构建时通过 -ldflags 注入，例如：
//
//	go build -ldflags "-X 'nekocode/internal/version.Version=v0.3.4'" ./cmd/tui
//
// release.yml 在打 tag 发布时自动注入 tag 名。本地直接 go build
// 不注入时，Version 保持默认值 "dev"，便于区分正式发布与本地构建。
package version

// Version 是当前构建的版本号。release.yml 在打 tag 发布时通过
// -X 覆盖为 tag 名（如 v0.3.4）；本地构建保持 "dev"。
var Version = "dev"
