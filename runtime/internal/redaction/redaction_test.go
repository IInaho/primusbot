package redaction

import "testing"

func TestRedactInputText(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/connect telegram token 123:secret-token", "/connect telegram token [redacted]"},
		{"/connect telegram add personal 123:secret-token", "/connect telegram add personal [redacted]"},
		{"/connect telegram add 123:secret-token", "/connect telegram add [redacted]"},
		{"/connect slack token xoxb-secret-token", "/connect slack token [redacted]"},
		{"/connect slack add workspace xoxb-secret-token", "/connect slack add workspace [redacted]"},
		{"/connect discord token secret-token", "/connect discord token [redacted]"},
		{"/connect telegram status", "/connect telegram status"},
		{"hello world", "hello world"},
	}

	for _, tc := range cases {
		got := RedactInputText(tc.input)
		if got != tc.want {
			t.Fatalf("RedactInputText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
