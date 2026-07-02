package permission

import "testing"

func TestBashMatcher(t *testing.T) {
	m := BashMatcher{}
	cases := []struct {
		spec string
		cmd  string
		want bool
	}{
		{"npm run *", "npm run build", true},
		{"npm run *", "npm run", true},
		{"npm run *", "npm install", false},
		{"npm run", "npm run", true},
		{"npm run", "npm run build", false},
		{"npm run:*", "npm run build", true},
		{"*", "anything here", true},
		{"ls *", "ls -la", true},
		{"ls *", "lsof", false},
		{"ls*", "lsof", true},
		{"git push *", "git push origin main", true},
		{"git push", "git push", true},
		{"rm *", "rm -rf /", true},
		{"sudo *", "sudo rm file", true},
		{"sudo", "sudo", true},
		{"sudo", "sudo rm file", false},
	}
	for _, c := range cases {
		got, err := m.Match(c.spec, map[string]any{"command": c.cmd})
		if err != nil {
			t.Fatalf("Match(%q,%q) err: %v", c.spec, c.cmd, err)
		}
		if got != c.want {
			t.Errorf("Match(%q,%q) = %v, want %v", c.spec, c.cmd, got, c.want)
		}
	}
}

func TestBashMatcherCompoundMustMatchAll(t *testing.T) {
	m := BashMatcher{}
	// "ls *" should NOT match "ls -la && rm -rf /" — rm is not allowed by the rule.
	got, _ := m.Match("ls *", map[string]any{"command": "ls -la && rm -rf /"})
	if got {
		t.Error("allow rule must match every subcommand; rm subcommand should fail")
	}
	// "npm *" should match "npm build && npm test" (both subcommands match).
	got, _ = m.Match("npm *", map[string]any{"command": "npm build && npm test"})
	if !got {
		t.Error("both subcommands match npm * → should match")
	}
}

func TestBashMatcherIsShellAware(t *testing.T) {
	m := BashMatcher{}
	got, _ := m.Match("echo *", map[string]any{"command": `echo "a && b"`})
	if !got {
		t.Fatal("quoted shell separators must not split the command")
	}
	got, _ = m.Match("grep *", map[string]any{"command": `grep "x|y" file.txt`})
	if !got {
		t.Fatal("quoted pipe must not split the command")
	}
}

func TestBashMatcherExactCompoundCommand(t *testing.T) {
	m := BashMatcher{}
	cmd := `echo "喵~ 你好！" && date && uname -a`
	got, _ := m.Match(cmd, map[string]any{"command": cmd})
	if !got {
		t.Fatal("exact remembered compound command should match the same command")
	}
	got, _ = m.Match(cmd, map[string]any{"command": `echo "喵~ 你好！" && date`})
	if got {
		t.Fatal("exact remembered compound command must not match a different command")
	}
}

func TestBashDenyMatchesAnySubcommand(t *testing.T) {
	info := map[string]any{"command": "ls -la && sudo rm -rf /tmp/x"}
	if !BashRuleMatches("sudo *", info, MatchAnySubcommand) {
		t.Fatal("deny-style matching should catch sudo in any subcommand")
	}
	if BashRuleMatches("ls *", info, MatchAllSubcommands) {
		t.Fatal("allow-style matching still requires every subcommand to match")
	}
}

func TestBashDenyMatchesCommandSubstitution(t *testing.T) {
	info := map[string]any{"command": "echo $(sudo rm -rf /tmp/x)"}
	if !BashRuleMatches("sudo *", info, MatchAnySubcommand) {
		t.Fatal("deny-style matching should catch sudo inside command substitution")
	}
}

func TestBashMatcherStripsWrappers(t *testing.T) {
	m := BashMatcher{}
	got, _ := m.Match("npm run *", map[string]any{"command": "timeout 30 npm run test"})
	if !got {
		t.Error("timeout wrapper should be stripped, then match npm run *")
	}
	got, _ = m.Match("git status", map[string]any{"command": "time git status"})
	if !got {
		t.Error("time wrapper should be stripped")
	}
}

func TestFilePathMatcher(t *testing.T) {
	m := FilePathMatcher{}
	info := map[string]any{"workspace": "/repo", "home": "/home/user"}
	cases := []struct {
		spec  string
		path  string
		want  bool
		extra map[string]any
	}{
		{"/src/**", "/repo/src/foo/bar.go", true, info},
		{"/src/**", "/repo/src/a.go", true, info},
		{"/src/**", "/repo/lib/x.go", false, info},
		{"~/Documents/*", "/home/user/Documents/notes.md", true, info},
		{"~/.ssh/*", "/home/user/.ssh/id_rsa", true, info},
		{"//tmp/scratch.txt", "/tmp/scratch.txt", true, info},
		{"*.env", "/repo/.env", true, info},
		{"*.env", "/repo/config/settings.env", true, info},
		{"*.env", "/repo/main.go", false, info},
		{"/src/*.ts", "/repo/src/app.ts", true, info},
		{"/src/*.ts", "/repo/src/sub/app.ts", false, info}, // * doesn't cross dirs
	}
	for _, c := range cases {
		got, err := m.Match(c.spec, mergeMaps(map[string]any{"path": c.path}, c.extra))
		if err != nil {
			t.Fatalf("Match(%q,%q) err: %v", c.spec, c.path, err)
		}
		if got != c.want {
			t.Errorf("Match(%q,%q) = %v, want %v", c.spec, c.path, got, c.want)
		}
	}
}

func TestDomainMatcher(t *testing.T) {
	m := DomainMatcher{}
	cases := []struct {
		spec   string
		domain string
		want   bool
	}{
		{"domain:github.com", "github.com", true},
		{"domain:github.com", "api.github.com", false},
		{"domain:*.github.com", "api.github.com", true},
		{"domain:*.github.com", "a.b.github.com", true},
		{"domain:*.github.com", "github.com", false},
		{"domain:*", "anything.com", true},
		{"domain:example.com.", "example.com", true}, // trailing dot stripped
		{"domain:example.com", "EXAMPLE.COM", true},  // case-insensitive
	}
	for _, c := range cases {
		got, _ := m.Match(c.spec, map[string]any{"domain": c.domain})
		if got != c.want {
			t.Errorf("Match(%q,%q) = %v, want %v", c.spec, c.domain, got, c.want)
		}
	}
}

func TestMCPMatcher(t *testing.T) {
	m := MCPMatcher{}
	cases := []struct {
		spec string
		tool string
		want bool
	}{
		{"mcp__github", "mcp__github", true},
		{"mcp__github__*", "mcp__github__create_issue", true},
		{"mcp__github__*", "mcp__github", false},
		{"mcp__github__create_issue", "mcp__github__create_issue", true},
		{"mcp__github__create_issue", "mcp__github__list_issues", false},
		{"mcp__github__create*", "mcp__github__create_issue", true},
	}
	for _, c := range cases {
		got, _ := m.Match(c.spec, map[string]any{"tool": c.tool})
		if got != c.want {
			t.Errorf("Match(%q,%q) = %v, want %v", c.spec, c.tool, got, c.want)
		}
	}
}

func TestBuiltinRulesNonEmpty(t *testing.T) {
	rules := BuiltinRules()
	if len(rules) < 20 {
		t.Fatalf("expected builtin rules, got %d", len(rules))
	}
	// every builtin rule must have a source and a valid effect
	for _, r := range rules {
		if r.Source != "builtin" {
			t.Errorf("builtin rule %+v has wrong source", r)
		}
		if r.Effect < EffectDeny || r.Effect > EffectAllow {
			t.Errorf("builtin rule %+v has invalid effect", r)
		}
	}
}

func TestDefaultMatchers(t *testing.T) {
	m := DefaultMatchers()
	if m["bash"] == nil {
		t.Fatal("bash matcher missing")
	}
	if m["edit"] == nil {
		t.Fatal("edit matcher missing")
	}
	if m["web_fetch"] == nil {
		t.Fatal("web_fetch matcher missing")
	}
	if m["mcp"] == nil {
		t.Fatal("mcp matcher missing")
	}
}

func mergeMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
