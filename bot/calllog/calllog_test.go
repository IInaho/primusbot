package calllog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSevereMiss(t *testing.T) {
	cases := []struct {
		hit, miss int
		want      bool
	}{
		{3_500, 348_501, true},    // the observed collapse: ~1% hit
		{0, 60_000, true},         // total loss just above threshold
		{0, 49_999, false},        // below the size threshold
		{100_000, 348_501, false}, // healthy hit ratio
	}
	for _, c := range cases {
		if got := SevereMiss(c.hit, c.miss); got != c.want {
			t.Errorf("SevereMiss(%d, %d) = %v, want %v", c.hit, c.miss, got, c.want)
		}
	}
}

func TestSinkWriteAppendsJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	s := &sink{path: path}

	s.write(Record{Source: "main", Model: "m1", CacheMissTokens: 10})
	s.write(Record{Source: "subagent", Err: "boom"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	var first, second Record
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 not JSON: %v", err)
	}
	if first.Source != "main" || first.Model != "m1" || first.CacheMissTokens != 10 {
		t.Errorf("record 1 = %+v", first)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 2 not JSON: %v", err)
	}
	if second.Err != "boom" {
		t.Errorf("record 2 = %+v", second)
	}
}

func TestBodyHashStable(t *testing.T) {
	body := []byte(`{"model":"m","messages":[]}`)
	if BodyHash(body) != BodyHash(append([]byte(nil), body...)) {
		t.Fatal("same bytes must hash equal")
	}
	if BodyHash(body) == BodyHash([]byte(`{"model":"m","messages":[{}]}`)) {
		t.Fatal("different bytes must hash differently")
	}
	if got := ShortDigest([32]byte{0xab, 0xcd, 0xef, 1, 2, 3}); got != "abcdef010203" {
		t.Fatalf("ShortDigest = %q", got)
	}
}
