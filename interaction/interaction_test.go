package interaction

import "testing"

func TestToolBriefParsesJSONToolArgs(t *testing.T) {
	if got := ToolBrief("edit", `{"path":"/tmp/a.go","oldString":"a","newString":"b"}`); got != "/tmp/a.go" {
		t.Fatalf("edit args = %q, want path", got)
	}
	if got := ToolBrief("shell", `{"command":"go test ./..."}`); got != "go test ./..." {
		t.Fatalf("shell args = %q, want command", got)
	}
}

func TestToolBriefReadLineRange(t *testing.T) {
	got := ToolBrief("read", `{"path":"/tmp/a.go","startLine":10,"endLine":20}`)
	if got != "/tmp/a.go 10-20" {
		t.Fatalf("read args = %q, want path with line range", got)
	}
}

func TestToolBriefShellSessionActions(t *testing.T) {
	if got := ToolBrief("shell", `{"action":"list"}`); got != "shell sessions" {
		t.Fatalf("shell list args = %q, want shell sessions", got)
	}
	if got := ToolBrief("shell", `{"action":"wait","session_id":3}`); got != "session 3" {
		t.Fatalf("shell wait args = %q, want session 3", got)
	}
	if got := ToolAction("shell", `{"action":"logs","session_id":3}`); got != "poll" {
		t.Fatalf("shell logs action = %q, want poll", got)
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
	cmd := `go test ./interaction/tui/... ./bot/... ./runtime/... ./interaction/gui/app/... ./bot/tools/... ./bot/agent/... ./bot/contextmgr/...`
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
