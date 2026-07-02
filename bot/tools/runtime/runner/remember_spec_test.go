package runner

import (
	"reflect"
	"testing"
)

func TestBashRememberSpecsUsesShellAST(t *testing.T) {
	cmd := `echo "喵~ bash 命令测试成功！当前工作目录: $(pwd)" && date`
	want := []string{"echo *", "pwd", "date"}
	if got := bashRememberSpecs(cmd); !reflect.DeepEqual(got, want) {
		t.Fatalf("bashRememberSpecs() = %#v, want %#v", got, want)
	}
}

func TestPathRememberSpecKeepsWorkspaceRootFileExact(t *testing.T) {
	if got := pathRememberSpec("/repo/go.mod", "/repo", "/home/user"); got != "/go.mod" {
		t.Fatalf("root workspace file should be remembered exactly, got %q", got)
	}
}
