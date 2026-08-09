package runner

import (
	"reflect"
	"testing"
)

func TestBashRememberRulesKeepsDynamicCommandLiteral(t *testing.T) {
	cmd := `echo "喵~ bash 命令测试成功！当前工作目录: $(pwd)" && date`
	got := bashRememberRules("shell", cmd)
	if len(got) != 1 || got[0].Literal != cmd || got[0].Specifier != "" {
		t.Fatalf("bashRememberRules() = %#v, want one exact literal", got)
	}
}

func TestBashRememberRulesBroadensOnlyStaticCommands(t *testing.T) {
	got := bashRememberRules("shell", `echo hello && date`)
	wantSpecs := []string{"echo *", "date"}
	var gotSpecs []string
	for _, rule := range got {
		gotSpecs = append(gotSpecs, rule.Specifier)
	}
	if !reflect.DeepEqual(gotSpecs, wantSpecs) {
		t.Fatalf("bashRememberRules() specs = %#v, want %#v", gotSpecs, wantSpecs)
	}
}

func TestPathRememberSpecKeepsWorkspaceRootFileExact(t *testing.T) {
	if got := pathRememberSpec("/repo/go.mod", "/repo", "/home/user"); got != "/go.mod" {
		t.Fatalf("root workspace file should be remembered exactly, got %q", got)
	}
}
