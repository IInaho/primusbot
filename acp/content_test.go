package acp

import "testing"

func TestPromptContent(t *testing.T) {
	text, err := promptText([]contentBlock{
		{Type: "text", Text: "Review this"},
		{Type: "resource_link", Name: "main.go", URI: "file:///repo/main.go", Description: "entrypoint"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "Review this\n\n[main.go](file:///repo/main.go) — entrypoint"
	if text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
	if _, err := promptText([]contentBlock{{Type: "image"}}); err == nil {
		t.Fatal("unsupported image content was accepted")
	}
}
