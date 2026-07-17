package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	controlruntime "nekocode/runtime"
	"nekocode/runtime/view"
)

type fakeRuntime struct {
	bus   *controlruntime.EventBus
	store *controlruntime.RunStore

	mu              sync.Mutex
	nextRun         int
	approved        string
	approvalAllowed bool
	answered        string
	answerRejected  bool
	aborted         controlruntime.RunID
	connectName     string
	connectArgs     []string
	disconnectName  string
}

func newFakeRuntime() *fakeRuntime {
	bus := controlruntime.NewEventBus()
	store := controlruntime.NewRunStore(0)
	bus.AddObserver(store.Record)
	return &fakeRuntime{bus: bus, store: store}
}

func (f *fakeRuntime) Submit(_ context.Context, input controlruntime.Input) (controlruntime.RunID, error) {
	f.mu.Lock()
	f.nextRun++
	runID := controlruntime.RunID("run_test")
	if f.nextRun > 1 {
		runID = controlruntime.RunID("run_test_" + string(rune('0'+f.nextRun)))
	}
	f.mu.Unlock()
	f.bus.Publish(controlruntime.Event{
		RunID:  runID,
		Type:   controlruntime.EventInputAccepted,
		Source: input.Source,
		Payload: controlruntime.MessagePayload{
			Role:    "user",
			Content: input.Text,
			Source:  input.Source,
			Sender:  input.Sender,
		},
	})
	f.bus.Publish(controlruntime.Event{RunID: runID, Type: controlruntime.EventRunStarted})
	return runID, nil
}

func (f *fakeRuntime) Steer(context.Context, controlruntime.RunID, controlruntime.Input) error {
	return nil
}

func (f *fakeRuntime) Abort(_ context.Context, runID controlruntime.RunID) error {
	f.mu.Lock()
	f.aborted = runID
	f.mu.Unlock()
	f.bus.Publish(controlruntime.Event{RunID: runID, Type: controlruntime.EventRunAborted})
	return nil
}

func (f *fakeRuntime) Approve(_ context.Context, approvalID string, decision controlruntime.ApprovalDecision) error {
	f.mu.Lock()
	f.approved = approvalID
	f.approvalAllowed = decision.Allowed
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntime) Answer(_ context.Context, questionID string, reply view.QuestionReply) error {
	f.mu.Lock()
	f.answered = questionID
	f.answerRejected = reply.Rejected
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntime) Subscribe(ctx context.Context, filter controlruntime.EventFilter) (<-chan controlruntime.Event, error) {
	return f.bus.Subscribe(ctx, filter)
}

func (f *fakeRuntime) SubscribeReplay(ctx context.Context, filter controlruntime.EventFilter) (<-chan controlruntime.Event, error) {
	return f.bus.SubscribeReplay(ctx, filter)
}

func (f *fakeRuntime) CurrentRunView() (controlruntime.RunView, bool) {
	return f.store.CurrentRunView()
}

func (f *fakeRuntime) RunView(runID controlruntime.RunID) (controlruntime.RunView, bool) {
	return f.store.RunView(runID)
}

func (f *fakeRuntime) ListRunViews(limit int) []controlruntime.RunView {
	return f.store.ListRunViews(limit)
}

func (f *fakeRuntime) ArtifactView(runID controlruntime.RunID) (controlruntime.ArtifactView, bool) {
	return f.store.ArtifactView(runID)
}

func (f *fakeRuntime) ConnectView() controlruntime.ConnectView {
	return controlruntime.ConnectView{
		Connectors: []controlruntime.ConnectorView{
			{
				Name:        "telegram",
				Registered:  true,
				Initialized: true,
				Configured:  true,
				Running:     true,
				Status:      "running",
				Devices: []controlruntime.ConnectorDeviceView{
					{ID: "1", Username: "alice"},
				},
			},
		},
	}
}

func (f *fakeRuntime) Connect(_ context.Context, name string, args []string) (string, error) {
	f.mu.Lock()
	f.connectName = name
	f.connectArgs = append([]string(nil), args...)
	f.mu.Unlock()
	return "connected " + name, nil
}

func (f *fakeRuntime) Disconnect(name string) (string, error) {
	f.mu.Lock()
	f.disconnectName = name
	f.mu.Unlock()
	return "disconnected " + name, nil
}

func (f *fakeRuntime) EventHistory(filter controlruntime.EventFilter) []controlruntime.Event {
	return f.bus.History(filter)
}

func (f *fakeRuntime) Stats() view.BotStats {
	return view.BotStats{PromptTokens: 10, CompletionTokens: 5}
}

func (f *fakeRuntime) ContextSnapshot() view.ContextSnapshot {
	return view.ContextSnapshot{Budget: 100, Used: 25, Free: 75}
}

func (f *fakeRuntime) ListSessions() []view.SessionMeta {
	return []view.SessionMeta{{ID: "session_1", CWD: "/tmp", MsgCount: 2}}
}

func (f *fakeRuntime) SessionMessages() []view.DisplayMessage {
	return []view.DisplayMessage{{Role: "user", Content: "hello"}}
}

func TestServerSubmitAndQueryRun(t *testing.T) {
	rt := newFakeRuntime()
	server := httptest.NewServer(New(rt).Handler())
	defer server.Close()

	resp := postJSON(t, server.URL+"/input", map[string]any{
		"text": "hello",
		"source": map[string]any{
			"kind": "test",
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var submitted submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&submitted); err != nil {
		t.Fatalf("decode submit: %v", err)
	}
	if submitted.RunID != "run_test" {
		t.Fatalf("run id = %q", submitted.RunID)
	}

	resp, err := http.Get(server.URL + "/runs/run_test")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("run status = %d", resp.StatusCode)
	}
	var run controlruntime.RunView
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.Input != "hello" || run.Source.Kind != "test" {
		t.Fatalf("run view = %#v", run)
	}
}

func TestServerConnectView(t *testing.T) {
	rt := newFakeRuntime()
	handler := New(rt).Handler()

	req := httptest.NewRequest(http.MethodGet, "/connect", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("connect status = %d", rec.Code)
	}
	var view controlruntime.ConnectView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode connect: %v", err)
	}
	if len(view.Connectors) != 1 || view.Connectors[0].Name != "telegram" || len(view.Connectors[0].Devices) != 1 {
		t.Fatalf("connect view = %#v", view)
	}
}

func TestServerExtraQueriesAndConnectCommand(t *testing.T) {
	rt := newFakeRuntime()
	handler := New(rt).Handler()

	req := httptest.NewRequest(http.MethodPost, "/input", bytes.NewBufferString(`{"text":"hello"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("input status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/runs/current", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("current run status = %d body=%s", rec.Code, rec.Body.String())
	}

	for _, path := range []string{"/stats", "/context", "/sessions", "/sessions/current/messages"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/connect/telegram", bytes.NewBufferString(`{"args":["status"]}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("connect command status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rt.connectName != "telegram" || len(rt.connectArgs) != 1 || rt.connectArgs[0] != "status" {
		t.Fatalf("connect command not forwarded: name=%q args=%#v", rt.connectName, rt.connectArgs)
	}

	req = httptest.NewRequest(http.MethodPost, "/connect/slack", http.NoBody)
	req.ContentLength = -1
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("connect empty body status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/disconnect/telegram", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("disconnect command status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rt.disconnectName != "telegram" {
		t.Fatalf("disconnect command not forwarded: %q", rt.disconnectName)
	}
}

func TestServerArtifactsAndControls(t *testing.T) {
	rt := newFakeRuntime()
	handler := New(rt).Handler()
	diff := "--- a/file\n+++ b/file\n@@\n-a\n+b"
	rt.bus.Publish(controlruntime.Event{
		RunID: "run_1",
		Type:  controlruntime.EventToolPreview,
		Payload: controlruntime.ToolPayload{
			ToolName: "edit",
			Preview:  diff,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/runs/run_1/artifacts", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("artifact status = %d body=%s", rec.Code, rec.Body.String())
	}
	var artifact controlruntime.ArtifactView
	if err := json.Unmarshal(rec.Body.Bytes(), &artifact); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	if len(artifact.Diffs) != 1 || artifact.Diffs[0].Content != diff {
		t.Fatalf("artifact = %#v", artifact)
	}

	req = httptest.NewRequest(http.MethodPost, "/approvals/apr_1", bytes.NewBufferString(`{"allowed":true}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("approval status = %d", rec.Code)
	}
	if rt.approved != "apr_1" || !rt.approvalAllowed {
		t.Fatalf("approval not forwarded")
	}

	req = httptest.NewRequest(http.MethodPost, "/questions/q_1", bytes.NewBufferString(`{"rejected":true}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("question status = %d", rec.Code)
	}
	if rt.answered != "q_1" || !rt.answerRejected {
		t.Fatalf("question not forwarded")
	}

	req = httptest.NewRequest(http.MethodPost, "/runs/run_1/abort", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("abort status = %d", rec.Code)
	}
	if rt.aborted != "run_1" {
		t.Fatalf("abort not forwarded: %q", rt.aborted)
	}
}

func TestServerEventsSSE(t *testing.T) {
	rt := newFakeRuntime()
	server := httptest.NewServer(New(rt).Handler())
	defer server.Close()

	rt.bus.Publish(controlruntime.Event{
		RunID:   "run_1",
		Type:    controlruntime.EventSystemMessage,
		Payload: controlruntime.MessagePayload{Content: "historical"},
	})

	req, err := http.NewRequest(http.MethodGet, server.URL+"/events?run_id=run_1&replay=1", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("events request: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type = %q", ct)
	}

	rt.bus.Publish(controlruntime.Event{
		RunID:   "run_1",
		Type:    controlruntime.EventRunDone,
		Payload: controlruntime.DonePayload{Output: "ok"},
	})

	buf := make([]byte, 512)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("read sse: %v", err)
	}
	got := string(buf[:n])
	if !strings.Contains(got, "historical") && !strings.Contains(got, "event: run_done") {
		t.Fatalf("sse = %q", got)
	}
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}
