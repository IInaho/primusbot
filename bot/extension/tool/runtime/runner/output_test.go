package runner

import "testing"

func TestFormatOutputNormalizesTerminalControlCharacters(t *testing.T) {
	input := "$ vite build\n\x1b[2Kvite v8\n\rtransforming..."
	want := "$ vite build\nvite v8\ntransforming..."
	if got := formatOutput("shell", input); got != want {
		t.Fatalf("formatOutput() = %q, want %q", got, want)
	}
}
