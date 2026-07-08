package session

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
	"nekocode/bot/policy/ledger"
)

func TestFormatSessionList(t *testing.T) {
	if got := FormatSessionList(nil); got != "No saved sessions." {
		t.Fatalf("empty list = %q", got)
	}
	got := FormatSessionList([]Meta{{ID: "s1", CWD: "/tmp/work", MsgCount: 2}})
	if !strings.Contains(got, "s1") || !strings.Contains(got, "2 msgs") || !strings.Contains(got, "/sessions <id>") {
		t.Fatalf("unexpected list: %q", got)
	}
	if got := ResumeSuccess("s1", 3); !strings.Contains(got, "s1") || !strings.Contains(got, "3 messages") {
		t.Fatalf("unexpected resume success: %q", got)
	}
	if got := ResumeFailed("s1", errors.New("boom")); !strings.Contains(got, "Failed to resume session s1") {
		t.Fatalf("unexpected resume failure: %q", got)
	}
}

func TestExportMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctx.json")
	gotPath, err := ExportMessages([]types.Message{{Role: "user", Content: "hi"}}, path)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q", gotPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"role": "user"`) {
		t.Fatalf("unexpected export: %s", data)
	}
	if got := ExportSuccess(path, 1); !strings.Contains(got, path) || !strings.Contains(got, "1 messages") {
		t.Fatalf("unexpected export success: %q", got)
	}
	if got := ExportFailed(errors.New("write file: boom")); !strings.Contains(got, "Failed to write file") {
		t.Fatalf("unexpected export failure: %q", got)
	}
}

func TestApplyContextSnapshot(t *testing.T) {
	sess := &Snapshot{}
	snap := ctxmgr.ManagerSnapshot{
		SystemPrompt:    "sys",
		Skills:          "skills",
		Memory:          "mem",
		Archive:         "arch",
		Messages:        []types.Message{{Role: "user", Content: "hi"}},
		CompactBoundary: 3,
		Budget:          100,
		Tracker: token.State{
			LastPromptTokens: 1000,
			LastCompTokens:   200,
			NewMessageTokens: 50,
			CacheHitTokens:   70,
			CacheMissTokens:  30,
			Sub: token.SubStats{
				Count:           2,
				TotalTokens:     500,
				CacheHitTokens:  40,
				CacheMissTokens: 60,
			},
		},
	}
	ApplyContextSnapshot(sess, snap, 10, 20, map[string]bool{"b": true, "a": true, "skip": false})

	if sess.SystemPrompt != "sys" || sess.ContextWindow != 100 || sess.PromptTokens != 10 || sess.CompletionTokens != 20 {
		t.Fatalf("session fields not applied: %+v", sess)
	}
	if !reflect.DeepEqual(sess.LoadedSkills, []string{"a", "b"}) {
		t.Fatalf("loaded skills = %+v", sess.LoadedSkills)
	}
	if sess.TrackerPrompt != 1000 || sess.TrackerCompletion != 200 || sess.TrackerNewTokens != 50 || sess.CacheHitTokens != 70 || sess.CacheMissTokens != 30 || sess.SubCount != 2 || sess.SubTokens != 500 || sess.SubCacheHit != 40 || sess.SubCacheMiss != 60 {
		t.Fatalf("tracker fields not applied: %+v", sess)
	}
}

func TestManagerSnapshot(t *testing.T) {
	sess := &Snapshot{
		SystemPrompt:      "sys",
		Skills:            "skills",
		Memory:            "mem",
		Archive:           "arch",
		CompactBoundary:   2,
		ContextWindow:     50,
		TrackerPrompt:     1000,
		TrackerCompletion: 200,
		TrackerNewTokens:  50,
		CacheHitTokens:    70,
		CacheMissTokens:   30,
		SubCount:          2,
		SubTokens:         500,
		SubCacheHit:       40,
		SubCacheMiss:      60,
	}
	got := ManagerSnapshot(sess)
	if got.SystemPrompt != "sys" || got.Skills != "skills" || got.Budget != 50 || got.CompactBoundary != 2 {
		t.Fatalf("snapshot mismatch: %+v", got)
	}
	if got.Tracker.CacheHitTokens != 70 || got.Tracker.CacheMissTokens != 30 || got.Tracker.Sub.Count != 2 || got.Tracker.Sub.TotalTokens != 500 {
		t.Fatalf("tracker mismatch: %+v", got.Tracker)
	}
	if got.Tracker.LastPromptTokens != 1000 || got.Tracker.LastCompTokens != 200 || got.Tracker.NewMessageTokens != 50 {
		t.Fatalf("tracker token state mismatch: %+v", got.Tracker)
	}
}

func TestManagerPersistsAndRestoresLedgerSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := &testContextStore{}
	want := ledger.Snapshot{
		ReadFiles:      []string{"/tmp/read.go"},
		ModifiedFiles:  []string{"/tmp/write.go"},
		ToolEventCount: 2,
	}
	var restored ledger.Snapshot
	m := NewManager(ManagerOptions{
		CWD:            "/tmp/work",
		Context:        ctx,
		LedgerSnapshot: func() ledger.Snapshot { return want },
		RestoreLedger:  func(s ledger.Snapshot) { restored = s },
	})
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	id := m.CurrentID()
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Ledger.ReadFiles, want.ReadFiles) || !reflect.DeepEqual(loaded.Ledger.ModifiedFiles, want.ModifiedFiles) {
		t.Fatalf("saved ledger = %+v, want %+v", loaded.Ledger, want)
	}
	if _, err := m.Resume(id); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.ReadFiles, want.ReadFiles) || !reflect.DeepEqual(restored.ModifiedFiles, want.ModifiedFiles) {
		t.Fatalf("restored ledger = %+v, want %+v", restored, want)
	}
}

type testContextStore struct {
	snap ctxmgr.ManagerSnapshot
}

func (s *testContextStore) Snapshot() ctxmgr.ManagerSnapshot {
	return s.snap
}

func (s *testContextStore) Restore(snap ctxmgr.ManagerSnapshot) {
	s.snap = snap
}

func (s *testContextStore) Build() []types.Message {
	return s.snap.Messages
}

func (s *testContextStore) Clear() {
	s.snap = ctxmgr.ManagerSnapshot{}
}
