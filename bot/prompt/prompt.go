package prompt

import (
	_ "embed"
	"runtime"
	"time"
)

//go:embed system_zh.md
var systemPrompt string

type Root struct {
	Path   string
	Access string
}

type Environment struct {
	Cwd              string
	Roots            []Root
	ManagedProcesses string
}

type EnvironmentProvider func() Environment

type Builder struct {
	staticPrefix string
	cwd          string
	now          func() time.Time
	osRelease    func() string
	env          EnvironmentProvider
}

func New(cwd string) *Builder {
	return &Builder{
		staticPrefix: systemPrompt,
		cwd:          cwd,
		now:          time.Now,
		osRelease:    OSRelease,
	}
}

func (b *Builder) SetEnvironmentProvider(p EnvironmentProvider) {
	b.env = p
}

// BuildStatic returns the cache-stable instruction prefix. This is the only
// part that should be saved in a session snapshot.
func (b *Builder) BuildStatic() string {
	return b.staticPrefix
}

// BuildEnvironment returns volatile runtime metadata. Callers should inject
// it on every model request rather than storing it in conversation history.
func (b *Builder) BuildEnvironment() string {
	info := Environment{Cwd: b.cwd}
	if b.env != nil {
		info = b.env()
		if info.Cwd == "" {
			info.Cwd = b.cwd
		}
	}
	if info.Cwd == "" && len(info.Roots) == 0 && info.ManagedProcesses == "" {
		return ""
	}
	now := b.now
	if now == nil {
		now = time.Now
	}
	osRel := b.osRelease
	if osRel == nil {
		osRel = func() string { return runtime.GOOS }
	}
	return FormatEnvironment(info, now().Format("2006-01-02"), "bash", osRel(), runtime.GOARCH)
}
