package message

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/styles"
	"nekocode/protocol"

	"github.com/charmbracelet/x/ansi"
)

func TestTelemetryRendersReadableTurnVocabulary(t *testing.T) {
	sty := styles.DefaultStyles()
	item := NewTelemetryMessageItem(&sty, protocol.Metrics{
		Duration: "16.3s", TurnTotal: 22_597, TurnInput: 21_865, TurnCached: 21_200, TurnNew: 665, TurnOutput: 732, TurnReasoning: 283, TurnCacheReported: true,
	})
	clean := ansi.Strip(item.Render(180))
	want := "↳ 16.3s · 总计 22.6k tok · 输入 21.9k · 缓存 21.2k · 未缓存 665 · 输出 732 · 推理 283 · 本次命中 96.96%"
	if !strings.Contains(clean, want) {
		t.Fatalf("full telemetry = %q, want %q", clean, want)
	}
}

func TestTelemetryCompactsAtNarrowWidths(t *testing.T) {
	sty := styles.DefaultStyles()
	item := NewTelemetryMessageItem(&sty, protocol.Metrics{
		TurnInput: 14_575, TurnCached: 14_500, TurnNew: 75, TurnOutput: 12, TurnCacheReported: true,
	})
	clean := ansi.Strip(item.Render(36))
	for _, want := range []string{"缓 14.5k", "新 75", "出 12", "命中 99.49%"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("compact telemetry missing %q: %q", want, clean)
		}
	}
	if strings.Contains(clean, "cached") || len([]rune(clean)) > 36 {
		t.Fatalf("compact telemetry overflowed or kept verbose labels: %q", clean)
	}
}

func TestTelemetryMarksUnknownCacheUsage(t *testing.T) {
	sty := styles.DefaultStyles()
	item := NewTelemetryMessageItem(&sty, protocol.Metrics{TurnInput: 1000, TurnOutput: 12})
	clean := ansi.Strip(item.Render(80))
	if !strings.Contains(clean, "缓存 —") || !strings.Contains(clean, "未缓存 —") || !strings.Contains(clean, "本次命中 —") {
		t.Fatalf("unknown cache usage looked like a real miss: %q", clean)
	}
	if strings.Contains(clean, "推理") {
		t.Fatalf("zero reasoning tokens should be omitted: %q", clean)
	}
}
