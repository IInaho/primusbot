package llmstream

import (
	"errors"
	"testing"
	"time"

	"nekocode/bot/calllog"
	"nekocode/bot/provider/types"
)

func TestMergeUsageKeepsSplitReports(t *testing.T) {
	var acc types.StreamUsage
	// Anthropic pattern: prompt/cache at message_start, completion at
	// message_delta — neither report alone carries all fields.
	mergeUsage(&acc, &types.StreamUsage{PromptTokens: 1000, CacheHitTokens: 800, CacheMissTokens: 200})
	mergeUsage(&acc, &types.StreamUsage{CompletionTokens: 42})
	if acc.PromptTokens != 1000 || acc.CacheHitTokens != 800 || acc.CacheMissTokens != 200 || acc.CompletionTokens != 42 {
		t.Fatalf("merged usage = %+v", acc)
	}
}

func captureRecord(t *testing.T) *calllog.Record {
	t.Helper()
	var got calllog.Record
	origWrite, origDump := writeRecord, dumpBody
	writeRecord = func(rec calllog.Record) { got = rec }
	dumpBody = func(rec *calllog.Record, body []byte) {}
	t.Cleanup(func() { writeRecord, dumpBody = origWrite, origDump })
	return &got
}

func TestWriteCallRecordAssemblesEvidence(t *testing.T) {
	got := captureRecord(t)

	start := time.Now().Add(-2 * time.Second)
	stream := &StreamResult{
		Usage:        types.StreamUsage{PromptTokens: 100, CacheHitTokens: 10, CacheMissTokens: 90, CompletionTokens: 5},
		FirstTokenAt: start.Add(300 * time.Millisecond),
		Request: &types.RequestMeta{
			Model: "m", Protocol: "openai", BaseURL: "https://x", BodySHA256: "abc", BodyBytes: 7,
		},
	}
	opts := LLMCallOptions{
		Source: "main",
		Diagnostics: func() calllog.PrefixDiag {
			return calllog.PrefixDiag{
				ChangedParts: []string{"tail/provider"},
				SystemHash:   "s", ToolsHash: "t", HistoryCount: 3, HistoryHash: "h",
			}
		},
	}
	writeCallRecord(opts, stream, start, nil)

	if got.Source != "main" || got.Model != "m" || got.Protocol != "openai" || got.BodySHA256 != "abc" || got.BodyBytes != 7 {
		t.Errorf("request evidence = %+v", got)
	}
	if got.PromptTokens != 100 || got.CacheHitTokens != 10 || got.CacheMissTokens != 90 || got.CompletionTokens != 5 {
		t.Errorf("usage evidence = %+v", got)
	}
	if got.TTFTMs != 300 || got.DurMs < 1000 {
		t.Errorf("timing = ttft %dms dur %dms", got.TTFTMs, got.DurMs)
	}
	if len(got.ChangedParts) != 1 || got.HistoryCount != 3 || got.Err != "" {
		t.Errorf("diag = %+v", got.PrefixDiag)
	}
}

func TestWriteCallRecordOnFailureKeepsErr(t *testing.T) {
	got := captureRecord(t)

	writeCallRecord(LLMCallOptions{}, &StreamResult{}, time.Now(), errors.New("stream idle timeout"))

	if got.Err != "stream idle timeout" {
		t.Errorf("err = %q", got.Err)
	}
	if got.Source != "unknown" {
		t.Errorf("source default = %q", got.Source)
	}
}
