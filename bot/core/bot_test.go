package core

import (
	"context"
	"strings"
	"testing"
)

func TestNewBuildsRunnableBot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Error(err)
		}
	})

	if b.ag == nil || b.cmd == nil || b.ext == nil || b.sess == nil {
		t.Fatal("New returned a partially initialized bot")
	}
	root, ok := b.CommandMenu(context.Background(), "/")
	if !ok || len(root.Items) == 0 {
		t.Fatal("New registered no commands")
	}
	menu, ok := b.CommandMenu(context.Background(), "/model")
	if !ok || menu.Title != "Choose model" || len(menu.Items) != len(b.cfg.Models) || !menu.Items[0].Submit {
		t.Fatalf("model command menu = %+v, %v", menu, ok)
	}
	effortMenu, ok := b.CommandMenu(context.Background(), "/effort")
	if !ok || effortMenu.Title != "Reasoning effort" || len(effortMenu.Items) != 4 {
		t.Fatalf("effort command menu = %+v, %v", effortMenu, ok)
	}
	if effortMenu.Items[0].Value != "/effort auto" || !effortMenu.Items[0].Submit ||
		!strings.Contains(effortMenu.Items[0].Description, "current") {
		t.Fatalf("default effort menu item = %+v", effortMenu.Items[0])
	}
	for _, input := range []string{"/plan", "/export", "/new", "/context", "/summarize"} {
		if _, ok := b.CommandMenu(context.Background(), input); ok {
			t.Fatalf("free-form or immediate command %q unexpectedly exposed a menu", input)
		}
	}
}
