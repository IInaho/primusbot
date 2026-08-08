package reasoning

import "testing"

func TestSettingsValues(t *testing.T) {
	tests := []struct {
		settings Settings
		wantReq  string
		wantEff  string
	}{
		{Settings{}, "auto", "auto"},
		{Settings{Requested: "high", Effort: "high"}, "high", "high"},
		{Settings{Requested: "high", Effort: "high", Disabled: true}, "high", "auto"},
		{Settings{Requested: "high", Effort: "high", Disabled: true, ThinkingMode: "enabled"}, "high", "none"},
	}
	for _, test := range tests {
		if got := test.settings.RequestedValue(); got != test.wantReq {
			t.Errorf("RequestedValue() = %q, want %q", got, test.wantReq)
		}
		if got := test.settings.EffectiveValue(); got != test.wantEff {
			t.Errorf("EffectiveValue() = %q, want %q", got, test.wantEff)
		}
	}
}
