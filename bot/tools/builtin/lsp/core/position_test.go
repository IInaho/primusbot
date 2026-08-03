package lspcore

import (
	"runtime"
	"strings"
	"testing"
)

func TestURIRoundtrip(t *testing.T) {
	paths := []string{"/home/u/a b.go", "/x/y.rs"}
	if runtime.GOOS == "windows" {
		paths = []string{`C:\Users\u\a b.go`, `D:\x\y.rs`}
	}
	for _, p := range paths {
		uri := pathToURI(p)
		if !strings.HasPrefix(uri, "file://") {
			t.Errorf("%q → %q is not a file URI", p, uri)
		}
		if got := uriToPath(uri); got != p {
			t.Errorf("roundtrip %q → %q", p, got)
		}
	}
}

func TestLocateEncoding(t *testing.T) {
	content := "package x\nαβ foo()\n" // line 2 has two 2-byte runes then a space
	u16, err := locate(content, 2, "foo", encodingUTF16)
	if err != nil {
		t.Fatal(err)
	}
	if u16.Line != 1 || u16.Character != 3 {
		t.Errorf("utf16 pos = %+v, want line 1 char 3", u16)
	}
	u8, err := locate(content, 2, "foo", encodingUTF8)
	if err != nil {
		t.Fatal(err)
	}
	if u8.Character != 5 {
		t.Errorf("utf8 char = %d, want 5", u8.Character)
	}
	if _, err := locate(content, 2, "missing", encodingUTF16); err == nil {
		t.Error("expected not-found error")
	}
	if _, err := locate(content, 99, "foo", encodingUTF16); err == nil {
		t.Error("expected out-of-range error")
	}
}

func TestLocateQualifiedSymbol(t *testing.T) {
	content := "package main\n\n\tgreet.Greet(\"x\")\n\tjson.loads('{}')\n"
	cases := []struct {
		line   int
		symbol string
		want   int
	}{
		{3, "greet.Greet", 7}, // tab + "greet."
		{3, "Greet", 7},       // same column without the qualifier
		{4, "json.loads", 6},  // tab + "json."
		{4, "loads", 6},
		{4, "pkg.loads", 6}, // qualifier differs from the line: fall back to "loads"
	}
	for _, c := range cases {
		pos, err := locate(content, c.line, c.symbol, encodingUTF16)
		if err != nil {
			t.Fatalf("locate(%q) line %d: %v", c.symbol, c.line, err)
		}
		if pos.Character != c.want {
			t.Errorf("locate(%q) line %d char = %d, want %d", c.symbol, c.line, pos.Character, c.want)
		}
	}
}
