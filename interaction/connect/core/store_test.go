package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFileStorePreservesOtherSections is the regression test for the
// historical bug where telegram's saveConfig rewrote the whole file and
// dropped every other connector's section.
func TestFileStorePreservesOtherSections(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "connect.json"))

	type telegramCfg struct {
		ActiveProfile string `json:"active_profile,omitempty"`
	}
	type feishuCfg struct {
		AppID string `json:"app_id,omitempty"`
	}

	if err := store.Save("telegram", telegramCfg{ActiveProfile: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("feishu", feishuCfg{AppID: "cli_a"}); err != nil {
		t.Fatal(err)
	}

	var tg telegramCfg
	if err := store.Load("telegram", &tg); err != nil {
		t.Fatal(err)
	}
	if tg.ActiveProfile != "main" {
		t.Fatalf("telegram section clobbered by feishu save: %+v", tg)
	}

	// And the reverse: saving telegram must keep the feishu section.
	if err := store.Save("telegram", telegramCfg{ActiveProfile: "alt"}); err != nil {
		t.Fatal(err)
	}
	var fs feishuCfg
	if err := store.Load("feishu", &fs); err != nil {
		t.Fatal(err)
	}
	if fs.AppID != "cli_a" {
		t.Fatalf("feishu section clobbered by telegram save: %+v", fs)
	}
	if tg.ActiveProfile = ""; store.Load("telegram", &tg) != nil || tg.ActiveProfile != "alt" {
		t.Fatalf("telegram section = %+v, want alt", tg)
	}
}

func TestFileStoreLoadMissing(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "connect.json"))
	var out struct {
		X string `json:"x"`
	}
	if err := store.Load("nope", &out); err != nil {
		t.Fatalf("missing file should load as zero, got %v", err)
	}

	// Unknown section in an existing file also loads as zero.
	if err := store.Save("other", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Load("nope", &out); err != nil {
		t.Fatalf("missing section should load as zero, got %v", err)
	}
}

func TestFileStoreRawRoundTrip(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "connect.json"))
	if err := store.Save("a", map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("b", map[string]int{"n": 2}); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2 {
		t.Fatalf("sections = %d, want 2", len(raw))
	}
}
