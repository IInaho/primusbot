package text

import "testing"

func TestNormalizeTerminalOutput(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"first\r\nsecond", "first\nsecond"},
		{"first\n\rsecond", "first\nsecond"},
		{"working\rdone", "working\ndone"},
	}
	for _, tt := range tests {
		if got := NormalizeTerminalOutput(tt.in); got != tt.want {
			t.Errorf("NormalizeTerminalOutput(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
