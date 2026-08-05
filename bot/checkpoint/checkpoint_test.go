package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestChangeKindKeepsManifestJSONCompatible(t *testing.T) {
	var item entry
	if err := json.Unmarshal([]byte(`{"path":"main.go","change":"modified"}`), &item); err != nil {
		t.Fatal(err)
	}
	if item.Change != ChangeModified {
		t.Fatalf("decoded change = %q, want %q", item.Change, ChangeModified)
	}
	data, err := json.Marshal(entry{Path: "main.go", Change: ChangeCreated})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"path":"main.go","before":"","change":"created"}` {
		t.Fatalf("encoded entry = %s", data)
	}
}

func TestRewindRestoresModifiedCreatedAndDeletedFiles(t *testing.T) {
	root := t.TempDir()
	work := t.TempDir()
	modified := filepath.Join(work, "modified.bin")
	created := filepath.Join(work, "created.txt")
	deleted := filepath.Join(work, "deleted.txt")
	original := []byte{0, 1, 2, '\r', '\n', 255}
	if err := os.WriteFile(modified, original, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deleted, []byte("restore me"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := New(root)
	m.Activate("session-1", nil, 0)
	turn, err := m.Begin("session-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{modified, created, deleted} {
		if err := m.Capture(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(modified, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(created, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(deleted); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{modified, created, deleted} {
		if err := m.Finalize(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Finish("session-1"); err != nil {
		t.Fatal(err)
	}
	history, err := m.History("session-1")
	if err != nil || len(history) != 1 || len(history[0].Changes) != 3 {
		t.Fatalf("history = %+v err=%v", history, err)
	}

	// Simulate session reload: only the persisted turn index is carried in
	// session.json; manifests and bytes remain on disk.
	reloaded := New(root)
	reloaded.Activate("session-1", m.Index("session-1"), m.Next("session-1"))
	result, err := reloaded.Rewind("session-1", turn)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 3 {
		t.Fatalf("restored files = %d, want 3", result.Files)
	}
	got, err := os.ReadFile(modified)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("modified bytes = %v, want %v", got, original)
	}
	info, err := os.Stat(modified)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("modified mode = %v, want 0640", info.Mode().Perm())
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("created file survived rewind: %v", err)
	}
	if got, err := os.ReadFile(deleted); err != nil || string(got) != "restore me" {
		t.Fatalf("deleted file restore = %q err=%v", got, err)
	}
	if index := reloaded.Index("session-1"); len(index) != 0 {
		t.Fatalf("rewound index = %v, want empty", index)
	}
	if next, err := reloaded.Begin("session-1"); err != nil || next != "2" {
		t.Fatalf("next turn after rewind = %q err=%v, want 2", next, err)
	}
}

func TestFinalizeDropsNoOpCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.txt")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(t.TempDir())
	m.Activate("session", nil, 0)
	turn, err := m.Begin("session")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Capture(path); err != nil {
		t.Fatal(err)
	}
	if err := m.Finalize(path); err != nil {
		t.Fatal(err)
	}
	if err := m.Finish("session"); err != nil {
		t.Fatal(err)
	}
	if turns := m.Index("session"); len(turns) != 0 {
		t.Fatalf("no-op turn was retained: %v", turns)
	}
	if _, err := os.Stat(m.turnDir("session", turn)); !os.IsNotExist(err) {
		t.Fatalf("no-op checkpoint directory survived: %v", err)
	}
	if _, err := m.Rewind("session", ""); err == nil {
		t.Fatal("rewind found an empty checkpoint")
	}
}

func TestBeginRejectsOverlappingTurns(t *testing.T) {
	m := New(t.TempDir())
	m.Activate("session", nil, 0)
	if _, err := m.Begin("session"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Begin("session"); err == nil {
		t.Fatal("second Begin replaced an active checkpoint turn")
	}
	if err := m.Finish("session"); err != nil {
		t.Fatal(err)
	}
	if turn, err := m.Begin("session"); err != nil || turn != "2" {
		t.Fatalf("Begin after Finish = %q, %v", turn, err)
	}
}

func TestFinishKeepsLatestTenCompleteTurns(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	m.Activate("session", nil, 0)
	for i := 1; i <= 12; i++ {
		if _, err := m.Begin("session"); err != nil {
			t.Fatal(err)
		}
		if err := m.Capture(path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := m.Finalize(path); err != nil {
			t.Fatal(err)
		}
		if err := m.Finish("session"); err != nil {
			t.Fatal(err)
		}
	}

	wantTurns := []string{"3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}
	if got := m.Index("session"); !reflect.DeepEqual(got, wantTurns) {
		t.Fatalf("retained turns = %v, want %v", got, wantTurns)
	}
	for _, turn := range []string{"1", "2"} {
		if _, err := os.Stat(m.turnDir("session", turn)); !os.IsNotExist(err) {
			t.Fatalf("pruned turn %s survived: %v", turn, err)
		}
	}
	history, err := m.History("session")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != MaxTurnsPerSession || history[0].Turn != "12" || history[len(history)-1].Turn != "3" {
		t.Fatalf("history order = %+v", history)
	}
}

func TestRewindRejectsInFlightMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(t.TempDir())
	m.Activate("session", nil, 0)
	if _, err := m.Begin("session"); err != nil {
		t.Fatal(err)
	}
	if err := m.Capture(path); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rewind("session", ""); err == nil {
		t.Fatal("rewind succeeded while mutation was in flight")
	}
	if err := os.WriteFile(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Finalize(path); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rewind("session", ""); err != nil {
		t.Fatal(err)
	}
}
