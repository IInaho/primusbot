package interaction

import "testing"

func TestToolBriefParsesJSONToolArgs(t *testing.T) {
	if got := ToolBrief("edit", `{"path":"/tmp/a.go","oldString":"a","newString":"b"}`); got != "/tmp/a.go" {
		t.Fatalf("edit args = %q, want path", got)
	}
	if got := ToolBrief("shell", `{"command":"go test ./..."}`); got != "go test ./..." {
		t.Fatalf("shell args = %q, want command", got)
	}
	if got := ToolBrief("task", `{"profile":"explore","skills":["check"],"prompt":"review policy"}`); got != "explore + [check] · review policy" {
		t.Fatalf("task args = %q, want profile and prompt", got)
	}
}

func TestToolBriefReadLineRange(t *testing.T) {
	got := ToolBrief("read", `{"path":"/tmp/a.go","startLine":10,"endLine":20}`)
	if got != "/tmp/a.go 10-20" {
		t.Fatalf("read args = %q, want path with line range", got)
	}
}

func TestToolBriefProcessActions(t *testing.T) {
	if got := ToolBrief("process", `{"action":"list"}`); got != "managed processes" {
		t.Fatalf("process list args = %q, want managed processes", got)
	}
	if got := ToolBrief("process", `{"action":"wait","task":"download"}`); got != "download" {
		t.Fatalf("process wait args = %q, want download", got)
	}
	if got := ToolAction("process", `{"action":"watch","task":"listener","event":"output"}`); got != "watch" {
		t.Fatalf("process watch action = %q, want watch", got)
	}
	if got := ToolAction("edit", `{"path":"/tmp/a.go"}`); got != "" {
		t.Fatalf("non-shell action = %q, want empty", got)
	}
}

func TestToolBriefKeepsPairSyntax(t *testing.T) {
	if got := ToolBrief("edit", `path=/tmp/a.go,oldString=a,newString=b`); got != "/tmp/a.go" {
		t.Fatalf("edit args = %q, want path", got)
	}
}

func TestToolBriefUnquotesBashCommandPreview(t *testing.T) {
	got := ToolBrief("shell", `command="echo \"Hello from bash! Current directory: $(pwd)\" && date"`)
	want := `echo "Hello from bash! Current directory: $(pwd)" && date`
	if got != want {
		t.Fatalf("shell args = %q, want %q", got, want)
	}
}

func TestToolBriefKeepsFullLongShellCommand(t *testing.T) {
	cmd := `go test ./interaction/tui/... ./bot/... ./runtime/... ./interaction/gui/app/... ./bot/extension/tool/... ./bot/agent/... ./bot/contextmgr/...`
	got := ToolBrief("shell", `{"command":"`+cmd+`"}`)
	if got != cmd {
		t.Fatalf("shell args should keep full command:\ngot  %q\nwant %q", got, cmd)
	}
}

func TestToolBriefTodoWrite(t *testing.T) {
	got := ToolBrief("todo_write", `{"todos":"[{\"content\":\"a\",\"status\":\"done\"},{\"content\":\"b\",\"status\":\"pending\"}]"}`)
	if got != "2 items" {
		t.Fatalf("todo_write args = %q, want 2 items", got)
	}
}
