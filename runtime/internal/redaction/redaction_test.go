package redaction

import "testing"

func TestRedactInputText(t *testing.T) {
	got := RedactInputText("/connect telegram token 123:secret-token")
	want := "/connect telegram token [redacted]"
	if got != want {
		t.Fatalf("redacted input = %q, want %q", got, want)
	}

	got = RedactInputText("/connect telegram add personal 123:secret-token")
	want = "/connect telegram add personal [redacted]"
	if got != want {
		t.Fatalf("redacted add input = %q, want %q", got, want)
	}

	got = RedactInputText("/connect telegram add 123:secret-token")
	want = "/connect telegram add [redacted]"
	if got != want {
		t.Fatalf("redacted short add input = %q, want %q", got, want)
	}
}
