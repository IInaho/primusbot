package common

import "testing"

func TestDangerLevelString(t *testing.T) {
	tests := []struct {
		level DangerLevel
		want  string
	}{
		{LevelSafe, "safe"},
		{LevelWrite, "modify"},
		{LevelDestructive, "danger"},
		{LevelForbidden, "blocked"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}
