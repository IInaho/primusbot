package telegram

import (
	"strings"
	"testing"
)

func TestTerminalQR(t *testing.T) {
	got, err := terminalQR("https://t.me/example_bot?start=test")
	if err != nil {
		t.Fatalf("terminalQR: %v", err)
	}
	if got == "" {
		t.Fatal("terminalQR returned empty output")
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatal("terminalQR should not add a trailing blank line")
	}
	if strings.Contains(got, "\n\n") {
		t.Fatal("terminalQR should not contain empty rows")
	}
	if !strings.Contains(got, "\x1b[47m") || !strings.Contains(got, "\x1b[40m") {
		t.Fatal("terminalQR should render explicit white and black backgrounds")
	}
}
