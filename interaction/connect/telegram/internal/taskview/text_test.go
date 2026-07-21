package taskview

import "testing"

func TestTruncateRunes(t *testing.T) {
	got := truncateRunes("你好世界", 2)
	if got != "你好..." {
		t.Fatalf("truncateRunes = %q", got)
	}
}
