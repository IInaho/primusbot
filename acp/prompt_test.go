package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestPrompt(t *testing.T) {
	backend := &fakeBackend{
		current:  "session-1",
		sessions: []controlruntime.SessionMeta{{ID: "session-1", CWD: "/workspace"}},
		events:   make(chan controlruntime.Event, 2),
	}
	var output bytes.Buffer
	s := &server{backend: backend, cwd: "/workspace", active: make(map[string]*activeTurn)}
	s.conn = newConnection(&output)
	result, rpcErr := s.prompt(context.Background(), json.RawMessage(`{"sessionId":"session-1","prompt":[{"type":"text","text":"hello"}]}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if result.(map[string]any)["stopReason"] != "end_turn" || backend.input.Text != "hello" || !backend.waited {
		t.Fatalf("result = %#v, input = %#v", result, backend.input)
	}
	if !strings.Contains(output.String(), `"sessionUpdate":"agent_message_chunk"`) {
		t.Fatalf("missing message update in %q", output.String())
	}
	select {
	case <-backend.eventCtx.Done():
	default:
		t.Fatal("prompt event subscription was not released")
	}
}
