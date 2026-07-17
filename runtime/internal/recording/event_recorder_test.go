package recording

import "testing"

func TestSafePathPart(t *testing.T) {
	got := safePathPart("../run:1")
	if got != "___run_1" {
		t.Fatalf("safePathPart = %q", got)
	}
}
