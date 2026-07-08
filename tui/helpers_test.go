package tui

import "testing"

func TestFormatBriefArgsParsesJSONToolArgs(t *testing.T) {
	if got := formatBriefArgs("edit", `{"path":"/tmp/a.go","oldString":"a","newString":"b"}`); got != "/tmp/a.go" {
		t.Fatalf("edit args = %q, want path", got)
	}
	if got := formatBriefArgs("shell", `{"command":"go test ./..."}`); got != "go test ./..." {
		t.Fatalf("shell args = %q, want command", got)
	}
}

func TestFormatBriefArgsKeepsPairSyntax(t *testing.T) {
	if got := formatBriefArgs("edit", `path=/tmp/a.go,oldString=a,newString=b`); got != "/tmp/a.go" {
		t.Fatalf("edit args = %q, want path", got)
	}
}

func TestFormatBriefArgsUnquotesBashCommandPreview(t *testing.T) {
	got := formatBriefArgs("shell", `command="echo \"Hello from bash! Current directory: $(pwd)\" && date"`)
	want := `echo "Hello from bash! Current directory: $(pwd)" && date`
	if got != want {
		t.Fatalf("shell args = %q, want %q", got, want)
	}
}

func TestFormatBriefArgsKeepsFullLongShellCommand(t *testing.T) {
	cmd := `go test ./tui/... ./bot/... ./common/... ./guiapp/... ./bot/tools/... ./bot/agent/... ./bot/contextmgr/...`
	got := formatBriefArgs("shell", `{"command":"`+cmd+`"}`)
	if got != cmd {
		t.Fatalf("shell args should keep full command:\ngot  %q\nwant %q", got, cmd)
	}
}
