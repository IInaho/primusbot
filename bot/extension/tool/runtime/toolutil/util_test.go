package toolutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nekocode/bot/extension/tool/runtime/workspace"
)

func TestStripAnsi(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello", "hello"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[?25lhidden cursor\x1b[?25h", "hidden cursor"},
		{"no ansi here", "no ansi here"},
	}
	for _, tt := range tests {
		if got := StripAnsi(tt.in); got != tt.want {
			t.Errorf("StripAnsi(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidatePath(t *testing.T) {
	td := t.TempDir()
	rel := filepath.Join(td, "sub")
	os.MkdirAll(rel, 0755)

	resolved, err := ValidatePath(rel)
	if err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
	if !filepath.IsAbs(resolved) {
		t.Errorf("expected absolute path, got %q", resolved)
	}

	// Non-existent path should still resolve (no error for non-existence).
	_, err = ValidatePath(filepath.Join(td, "nonexistent"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePathWritableUsesWorkspaceGuard(t *testing.T) {
	ws := t.TempDir()
	ctx := workspace.WithManager(context.Background(), workspace.New(ws, nil))

	inside := filepath.Join(ws, "a.txt")
	if _, err := ValidatePathWritableContext(ctx, inside); err != nil {
		t.Fatalf("inside workspace should be writable: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "evil.txt")
	if _, err := ValidatePathWritableContext(ctx, outside); err == nil {
		t.Fatal("outside workspace should be rejected")
	}
}

func TestNewToolHTTPClient(t *testing.T) {
	c := NewToolHTTPClient(0)
	if c == nil {
		t.Fatal("nil client")
	}
	if c.Timeout != 0 {
		t.Error("expected zero timeout")
	}
}

func TestComputeFileHash(t *testing.T) {
	hash := ComputeFileHash("hello\nworld\n")
	if len(hash) != 8 {
		t.Fatalf("expected 8-char hash, got %q", hash)
	}
	hash2 := ComputeFileHash("hello\nworld\n")
	if hash != hash2 {
		t.Fatalf("same content should produce same hash: %q vs %q", hash, hash2)
	}
	hash3 := ComputeFileHash("hello\nworld!\n")
	if hash == hash3 {
		t.Fatalf("different content should produce different hash: %q vs %q", hash, hash3)
	}
}

func TestComputeFileHash_CRLF(t *testing.T) {
	hashLF := ComputeFileHash("hello\nworld\n")
	hashCRLF := ComputeFileHash("hello\r\nworld\r\n")
	if hashLF != hashCRLF {
		t.Fatalf("CRLF/LF should produce same hash: %q vs %q", hashLF, hashCRLF)
	}
}

func TestComputeFileHash_TrailingWhitespace(t *testing.T) {
	hash1 := ComputeFileHash("hello\nworld\n")
	hash2 := ComputeFileHash("hello  \nworld\t\n")
	if hash1 != hash2 {
		t.Fatalf("trailing whitespace should not affect hash: %q vs %q", hash1, hash2)
	}
}
