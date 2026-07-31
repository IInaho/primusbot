package prompt

import (
	_ "embed"
	"runtime"
	"strings"
	"time"
)

//go:embed system_zh.md
var systemPrompt string

type Builder struct {
	staticPrefix string
	cwd          string
	now          func() time.Time
	osRelease    func() string
}

func New(cwd string) *Builder {
	return &Builder{
		staticPrefix: systemPrompt,
		cwd:          cwd,
		now:          time.Now,
		osRelease:    OSRelease,
	}
}

func (b *Builder) Build() string {
	var parts []string
	if b.staticPrefix != "" {
		parts = append(parts, b.staticPrefix)
	}
	if b.cwd != "" {
		now := b.now
		if now == nil {
			now = time.Now
		}
		osRel := b.osRelease
		if osRel == nil {
			osRel = func() string { return runtime.GOOS }
		}
		date := now().Format("2006-01-02")
		parts = append(parts, FormatEnv(b.cwd, date, osRel(), runtime.GOARCH))
	}
	return strings.Join(parts, "\n\n")
}
