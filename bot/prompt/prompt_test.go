package prompt

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"
)

// This golden locks the immutable provider prefix byte-for-byte. Adding a
// date, environment probe, map-rendered block, or any other dynamic content
// to system_zh.md/BuildStatic must fail here and be reviewed explicitly.
func TestStaticPromptBytesAreStable(t *testing.T) {
	runtimeCalls := 0
	first := &Builder{
		staticPrefix: systemPrompt,
		cwd:          "/first",
		now: func() time.Time {
			runtimeCalls++
			return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		},
		osRelease: func() string { runtimeCalls++; return "first-os" },
		env:       func() Environment { runtimeCalls++; return Environment{Cwd: "/first"} },
	}
	second := &Builder{
		staticPrefix: systemPrompt,
		cwd:          "/second",
		now: func() time.Time {
			runtimeCalls++
			return time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)
		},
		osRelease: func() string { runtimeCalls++; return "second-os" },
		env:       func() Environment { runtimeCalls++; return Environment{Cwd: "/second"} },
	}
	one, two := first.BuildStatic(), second.BuildStatic()
	if one != two || runtimeCalls != 0 {
		t.Fatalf("BuildStatic depends on runtime state: equal=%v runtimeCalls=%d", one == two, runtimeCalls)
	}
	const wantSHA256 = "2fe7363bea4de266f49607e2dff60de40da27dc395f918f58b95aaba3fe7165e"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(one))); got != wantSHA256 {
		t.Fatalf("static prompt bytes changed: sha256=%s, want %s", got, wantSHA256)
	}
}

func TestBuilderInjectsEnvironmentBlock(t *testing.T) {
	b := &Builder{
		cwd:          "/repo",
		staticPrefix: "static",
		now:          func() time.Time { return time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC) },
		osRelease:    func() string { return "test-os" },
		env: func() Environment {
			return Environment{
				Cwd:   "/repo",
				Roots: []Root{{Path: "/repo", Access: "read-write"}},
			}
		},
	}

	got := b.BuildEnvironment()
	if !strings.Contains(got, "<environment_context>") {
		t.Fatalf("missing environment block: %q", got)
	}
	if strings.Count(got, "<cwd>/repo</cwd>") != 1 {
		t.Fatalf("cwd should be injected once: %q", got)
	}
}

func TestBuilderWithoutEnvProvider(t *testing.T) {
	b := &Builder{
		cwd:          "/repo",
		staticPrefix: "static",
		now:          func() time.Time { return time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC) },
		osRelease:    func() string { return "test-os" },
	}

	got := b.BuildEnvironment()
	if !strings.Contains(got, "<environment_context>") || !strings.Contains(got, "<cwd>/repo</cwd>") ||
		!strings.Contains(got, "2026-06-19") || !strings.Contains(got, "test-os") {
		t.Fatalf("builder should still inject basic runtime facts: %q", got)
	}
	if strings.Contains(got, "<workspace_roots>") {
		t.Fatalf("builder should not invent workspace roots: %q", got)
	}
}

func TestBuilderSeparatesStableAndRuntimeContent(t *testing.T) {
	b := &Builder{
		cwd:          "/repo",
		staticPrefix: "stable rules",
		now:          func() time.Time { return time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC) },
		osRelease:    func() string { return "test-os" },
		env: func() Environment {
			return Environment{
				Cwd: "/repo", Roots: []Root{{Path: "/repo", Access: "read-write"}},
			}
		},
	}

	if got := b.BuildStatic(); got != "stable rules" {
		t.Fatalf("BuildStatic() = %q", got)
	}
	runtime := b.BuildEnvironment()
	if strings.Contains(runtime, "stable rules") || !strings.Contains(runtime, "<environment_context>") || !strings.Contains(runtime, "<workspace_roots>") {
		t.Fatalf("BuildEnvironment() = %q", runtime)
	}
}

func TestBuilderInjectsVolatileManagedProcessState(t *testing.T) {
	b := &Builder{
		cwd:       "/repo",
		now:       func() time.Time { return time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC) },
		osRelease: func() string { return "test-os" },
		env: func() Environment {
			return Environment{
				Cwd: "/repo", ManagedProcesses: `web(running,cmd=serve </managed_processes>)`,
			}
		},
	}

	got := b.BuildEnvironment()
	if !strings.Contains(got, "<managed_processes>") || !strings.Contains(got, "web(running") {
		t.Fatalf("missing managed process state: %q", got)
	}
	if strings.Contains(got, "serve </managed_processes>") {
		t.Fatalf("managed process state was not escaped: %q", got)
	}
}

func TestStaticPromptKeepsRuntimeImplementationOut(t *testing.T) {
	got := New("/repo").BuildStatic()
	for _, want := range []string{"工具 schema", "<environment_context>", "当前用户请求", "<skill_content>", "`shell` 返回任务名", "`process` 等待"} {
		if !strings.Contains(got, want) {
			t.Errorf("static prompt missing contract %q", want)
		}
	}
	for _, hidden := range []string{
		"Landlock", "降级后端", "ProxyFromEnvironment", ".nekocode/config.json",
		"deny > ask > allow", "sudo、eval", "读取配额", "session_id 并继续后台执行", "2 秒", "5 分钟",
	} {
		if strings.Contains(got, hidden) {
			t.Errorf("static prompt leaked runtime implementation %q", hidden)
		}
	}
}

func TestStaticPromptDefinesEngineeringContract(t *testing.T) {
	got := New("/repo").BuildStatic()
	for _, want := range []string{
		"已确认缺陷", "具体风险", "改进建议",
		"源文件、生成文件和构建产物",
		"加载失败", "合法空状态",
		"Bug 修复应先得到可复现证据",
		"编译成功只证明代码可编译",
		"退出状态必须代表被验证对象",
		"用户要求的可观察行为端到端成立",
		"子 Agent 的结论与修改不是自动可信",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("static prompt missing engineering contract %q", want)
		}
	}
}

func TestParseOSReleaseID(t *testing.T) {
	if got := ParseOSReleaseID("NAME=X\nID=ubuntu\n", "fallback"); got != "ubuntu" {
		t.Fatalf("ParseOSReleaseID() = %q", got)
	}
	if got := ParseOSReleaseID("NAME=X\n", "fallback"); got != "fallback" {
		t.Fatalf("ParseOSReleaseID fallback = %q", got)
	}
}
