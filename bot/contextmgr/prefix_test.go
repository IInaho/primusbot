package contextmgr

import (
	"reflect"
	"testing"

	"nekocode/bot/provider/types"
)

func TestPrefixShapeTreatsHistoryAppendAndMapOrderAsStable(t *testing.T) {
	system := []types.Message{{Role: "system", Content: "stable"}}
	history := []types.Message{{Role: "user", Content: "first"}}
	toolsA := []types.ToolDef{{Function: types.FunctionDef{Parameters: types.Parameters{Properties: map[string]types.Property{
		"b": {Type: "string"}, "a": {Type: "number"},
	}}}}}
	toolsB := []types.ToolDef{{Function: types.FunctionDef{Parameters: types.Parameters{Properties: map[string]types.Property{
		"a": {Type: "number"}, "b": {Type: "string"},
	}}}}}

	first := buildPrefixShape(system, history, toolsA)
	second := buildPrefixShape(system, append(history, types.Message{Role: "assistant", Content: "second"}), toolsB)
	if parts := changedPrefixParts(first, second); len(parts) != 0 {
		t.Fatalf("append-only request changed stable prefix: %v", parts)
	}
}

func TestPrefixShapeAttributesSystemToolsAndHistory(t *testing.T) {
	first := buildPrefixShape(
		[]types.Message{{Role: "system", Content: "one"}},
		[]types.Message{{Role: "user", Content: "one"}},
		[]types.ToolDef{{Function: types.FunctionDef{Name: "one"}}},
	)
	second := buildPrefixShape(
		[]types.Message{{Role: "system", Content: "two"}},
		[]types.Message{{Role: "user", Content: "rewritten"}},
		[]types.ToolDef{{Function: types.FunctionDef{Name: "two"}}},
	)
	if got, want := changedPrefixParts(first, second), []string{"system", "tools", "history"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("changed parts = %v, want %v", got, want)
	}
}

func TestBuildTwoRoundsKeepsVolatileLayersOutsideStablePrefix(t *testing.T) {
	runtimePrompt := "runtime-one"
	m := New(Config{
		SystemPrompt: "stable", ContextWindow: 10_000,
		RuntimePrompt: func() string { return runtimePrompt },
	})
	m.Add("user", "request")
	tools := []types.ToolDef{{Function: types.FunctionDef{Name: "read"}}}

	first := m.BuildRequest(tools)
	runtimePrompt = "runtime-two"
	m.SetHints("turn hint")
	second := m.BuildRequest(tools)
	m.RecordCache(100, 12)

	if first[0].Content != second[0].Content || first[1].Content != second[1].Content {
		t.Fatalf("volatile tail changed stable Build prefix:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if second[len(second)-2].Source != types.MessageSourceVolatileTail || second[len(second)-1].Source != types.MessageSourceVolatileTail {
		t.Fatalf("runtime/hint tail markers missing: %+v", second)
	}
	miss := m.Report().PrefixTurn.PeakMiss
	if miss.MissTokens != 12 || !reflect.DeepEqual(miss.Parts, []string{"tail/provider"}) {
		t.Fatalf("prefix diagnosis = %+v", miss)
	}
}

func TestPlainBuildStripsProviderRoutingMetadata(t *testing.T) {
	m := New(Config{SystemPrompt: "stable", RuntimePrompt: func() string { return "runtime" }})
	for _, message := range m.Build() {
		if message.Source != "" {
			t.Fatalf("plain Build leaked routing source %q", message.Source)
		}
	}
}

func TestPrefixTrackerReportsColdStartAndRewrittenHistory(t *testing.T) {
	var tracker prefixTracker
	tracker.BeginTurn()
	first := buildPrefixShape(nil, []types.Message{{Role: "user", Content: "one"}}, nil)
	tracker.Observe(first)
	tracker.RecordCache(0, 20)
	if got := tracker.TurnStats().PeakMiss; !reflect.DeepEqual(got.Parts, []string{"cold-start"}) {
		t.Fatalf("cold start = %+v", got)
	}

	rewritten := buildPrefixShape(nil, []types.Message{{Role: "user", Content: "two"}}, nil)
	tracker.Observe(rewritten)
	tracker.RecordCache(100, 15)
	stats := tracker.TurnStats()
	if stats.Requests != 2 || stats.HitTokens != 100 || stats.MissTokens != 35 {
		t.Fatalf("turn totals = %+v", stats)
	}
	if stats.PeakMiss.Request != 1 || stats.PeakMiss.MissTokens != 20 || !reflect.DeepEqual(stats.PeakMiss.Parts, []string{"cold-start"}) {
		t.Fatalf("worst miss = %+v", stats)
	}
	if stats.LowestHit.Request != 1 || stats.LowestHit.HitTokens != 0 {
		t.Fatalf("lowest hit = %+v", stats.LowestHit)
	}
}

func TestPrefixTrackerBeginsTurnWithoutLosingShapeBaseline(t *testing.T) {
	var tracker prefixTracker
	first := buildPrefixShape(nil, []types.Message{{Role: "user", Content: "one"}}, nil)
	tracker.Observe(first)
	tracker.RecordCache(0, 50)

	tracker.BeginTurn()
	second := buildPrefixShape(nil, []types.Message{
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "answer"},
	}, nil)
	tracker.Observe(second)
	tracker.RecordCache(90, 10)

	stats := tracker.TurnStats()
	if stats.Requests != 1 || stats.HitTokens != 90 || stats.MissTokens != 10 {
		t.Fatalf("new turn totals = %+v", stats)
	}
	if !reflect.DeepEqual(stats.PeakMiss.Parts, []string{"tail/provider"}) {
		t.Fatalf("shape baseline was lost: %+v", stats.PeakMiss)
	}
}

func TestPrefixTrackerKeepsPeakMissAndLowestHitSeparately(t *testing.T) {
	var tracker prefixTracker
	tracker.BeginTurn()
	shape := buildPrefixShape(nil, nil, nil)
	tracker.Observe(shape)
	tracker.RecordCache(900, 100) // 90% hit, largest miss.
	tracker.Observe(shape)
	tracker.RecordCache(80, 20) // 80% hit, lowest rate.

	stats := tracker.TurnStats()
	if stats.PeakMiss.Request != 1 || stats.PeakMiss.MissTokens != 100 {
		t.Fatalf("peak miss = %+v", stats.PeakMiss)
	}
	if stats.LowestHit.Request != 2 || stats.LowestHit.HitTokens != 80 || stats.LowestHit.MissTokens != 20 {
		t.Fatalf("lowest hit = %+v", stats.LowestHit)
	}
}
