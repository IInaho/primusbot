package edit

import (
	"nekocode/bot/extension/tool/runtime/toolutil"
	"testing"
)

func TestNormalizeToLF(t *testing.T) {
	if got := toolutil.NormalizeToLF("a\r\nb\rc\n"); got != "a\nb\nc\n" {
		t.Fatalf("got %q", got)
	}
}
