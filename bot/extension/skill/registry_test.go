package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	td := t.TempDir()
	sd := filepath.Join(td, "s1")
	os.MkdirAll(sd, 0755)
	os.WriteFile(filepath.Join(sd, "SKILL.md"), []byte(`---
name: s1
description: first skill
---

# Body
`), 0644)

	reg := newRegistry()
	reg.Load([]string{td})

	// Get.
	sk, ok := reg.Get("s1")
	if !ok || sk.Name != "s1" {
		t.Fatal("Get failed")
	}
	if _, ok := reg.Get("nope"); ok {
		t.Error("expected false for missing skill")
	}

	// List.
	if len(reg.List()) != 1 {
		t.Errorf("List = %d", len(reg.List()))
	}

	// MarkLoaded / IsLoaded / ClearLoaded / LoadedSet.
	reg.MarkLoaded("s1")
	if !reg.IsLoaded("s1") {
		t.Error("expected loaded")
	}
	if len(reg.LoadedSet()) != 1 {
		t.Error("LoadedSet wrong")
	}
	reg.ClearLoaded()
	if reg.IsLoaded("s1") {
		t.Error("expected not loaded after clear")
	}

	// RegisterAll (bundled).
	bundled := []*Skill{{Name: "b", Description: "bundled"}}
	reg.RegisterAll(bundled)
	if sk, _ := reg.Get("b"); sk == nil {
		t.Error("bundled skill not registered")
	}
}
