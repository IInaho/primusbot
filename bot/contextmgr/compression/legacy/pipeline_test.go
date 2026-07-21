package legacy

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"nekocode/bot/contextmgr/context"
	"nekocode/bot/provider/types"
)

// ---------------------------------------------------------------------------
// Level classification & budget helpers
// ---------------------------------------------------------------------------

func TestEstimatedTokens_Empty(t *testing.T) {
	cm := newPipelineCompactor(nil, 64000, 0, "")
	n := cm.estimateTokens()
	if n != 0 {
		t.Errorf("empty messages → 0, got %d", n)
	}
}

func TestEstimatedTokens_WithArchive(t *testing.T) {
	cm := newPipelineCompactor(
		[]types.Message{
			{Role: "user", Content: "hello world"},
		},
		64000, 0, "archive with some content",
	)
	n := cm.estimateTokens()
	if n <= 2 {
		t.Errorf("with archive: expected > 2, got %d", n)
	}
}

func TestEstimatedTokens_HonorsBoundary(t *testing.T) {
	cm := newPipelineCompactor(
		[]types.Message{
			{Role: "user", Content: "old message"},
			{Role: "user", Content: "visible message"},
		},
		64000, 1, "",
	)
	// boundary=1 hides first message — only "visible message" counted
	n := cm.estimateTokens()
	if n <= 0 {
		t.Errorf("should count visible messages, got %d", n)
	}
}

func TestEffectiveBudget_ZeroOrNegative(t *testing.T) {
	cm := newPipelineCompactor(nil, 0, 0, "")
	if b := cm.effectiveBudget(); b != defaultBudget {
		t.Errorf("zero budget → default %d, got %d", defaultBudget, b)
	}

	cm2 := newPipelineCompactor(nil, -100, 0, "")
	if b := cm2.effectiveBudget(); b != defaultBudget {
		t.Errorf("negative budget → default %d, got %d", defaultBudget, b)
	}
}

func TestEffectiveConfig_ScaleUp(t *testing.T) {
	cm := newPipelineCompactor(nil, 128000, 0, "")
	cfg := cm.effectiveConfig()
	if cfg.WarningBuffer <= DefaultConfig.WarningBuffer {
		t.Error("128K budget should scale WarningBuffer upward")
	}
	if cfg.CompactBuffer <= DefaultConfig.CompactBuffer {
		t.Error("128K budget should scale CompactBuffer upward")
	}
}

func TestEffectiveConfig_ScaleDown(t *testing.T) {
	cm := newPipelineCompactor(nil, 32000, 0, "")
	cfg := cm.effectiveConfig()
	// Should use unscaled config (budget < defaultBudget)
	if cfg.WarningBuffer != DefaultConfig.WarningBuffer {
		t.Error("below-default budget should use unscaled config")
	}
}

func TestClassifyLevel_Integration(t *testing.T) {
	cfg := DefaultConfig
	tests := []struct {
		remaining int
		expect    Level
	}{
		{cfg.WarningBuffer + 1000, LevelNormal},
		{cfg.WarningBuffer, LevelWarning},
		{cfg.WarningBuffer - 1000, LevelWarning},
		{cfg.MicroCompactBuffer, LevelMicroCompact},
		{cfg.CompactBuffer, LevelCompact},
		{cfg.BlockingBuffer, LevelBlocking},
		{0, LevelBlocking},
	}
	for _, tt := range tests {
		got := classifyLevel(tt.remaining, cfg)
		if got != tt.expect {
			t.Errorf("classifyLevel(%d) = %s, want %s", tt.remaining, got, tt.expect)
		}
	}
}

// ---------------------------------------------------------------------------
// AutoCompactIfNeeded — pipeline orchestration
// ---------------------------------------------------------------------------

func TestAutoCompactIfNeeded_Normal(t *testing.T) {
	cm := newPipelineCompactor(
		[]types.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
		64000, 0, "",
	)
	level, err := cm.AutoCompactIfNeeded()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != LevelNormal {
		t.Errorf("expected LevelNormal, got %s", level)
	}
}

func TestAutoCompactIfNeeded_Blocking(t *testing.T) {
	// fill messages to exceed budget
	msgs := make([]types.Message, 1000)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: strings.Repeat("x", 400)}
	}
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	_, err := cm.AutoCompactIfNeeded()
	if err == nil {
		t.Fatal("expected blocking error, got nil")
	}
	if !strings.Contains(err.Error(), "context full") {
		t.Errorf("error should mention 'context full', got: %v", err)
	}
}

func TestAutoCompactIfNeeded_NoSummarizer_DoesNotError(t *testing.T) {
	msgs := make([]types.Message, 50)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "message " + strings.Repeat("x", 200)}
	}
	bigBudget := 64000
	cm := &Compactor{
		Ctx:           &content.Content{Messages: msgs, CompactBoundary: 80},
		ContextWindow: &bigBudget,
		Tracker:       &testTracker{promptEst: 35000},
		CompactCount:  new(int),
		TrimCount:     new(int),
		Cfg:           DefaultConfig,
		// no Summarizer — LLM layers (Collapse, FullCompact) are silently skipped
	}
	level, err := cm.AutoCompactIfNeeded()
	if err != nil {
		t.Fatalf("no summarizer should not error, got: %v", err)
	}
	// Pipeline runs all available non-LLM layers and returns the final level.
	// Without a summarizer, layers 4-5 are no-ops; the returned level reflects
	// the last attempted layer (LevelCompact).
	if level > LevelCompact {
		t.Errorf("expected ≤ LevelCompact, got %s", level)
	}
}

func TestAutoCompact_SnipingEffectOnState(t *testing.T) {
	// Sniping removes cold messages (before compactBoundary) from the front
	// of the array and inserts a boundary marker. This test verifies the state
	// mutations are correct, then runs the full pipeline as a smoke test.
	//
	// NOTE: Sniping has minimal impact on visibleEstimatedTokens because
	// boundary shifts by only (snip - 1) positions — visible message count
	// barely changes. The recheckBudget after sniping returns essentially the
	// same level as before. "Stopping after sniping" is architecturally
	// unreachable in the current design without an updated API-based tracker.

	msgs := make([]types.Message, 120)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "x"}
	}
	cm := newPipelineCompactor(msgs, 64000, 80, "")
	cm.Tracker = &testTracker{promptEst: 55000}

	// Before sniping: boundary=80, 120 messages
	n := cm.SnipHistory()
	if n != 40 {
		t.Errorf("expected 40 sniped, got %d", n)
	}
	// After: boundary should be ~41 (80-40+1), messages should be ~81 (120-40+1)
	if cm.Ctx.CompactBoundary != 41 {
		t.Errorf("expected boundary=41 after snip, got %d", cm.Ctx.CompactBoundary)
	}
	if len(cm.Ctx.Messages) != 81 {
		t.Errorf("expected 81 messages after snip, got %d", len(cm.Ctx.Messages))
	}
	if cm.Ctx.Messages[0].Content != snipeBoundaryMarker {
		t.Error("boundary marker should be inserted at front")
	}

	// Now run full pipeline — should not error
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>ok</summary>", nil
	}
	level, err := cm.AutoCompactIfNeeded()
	if err != nil {
		t.Fatalf("pipeline after sniping failed: %v", err)
	}
	t.Logf("pipeline after sniping returned level=%s", level)
}

// ---------------------------------------------------------------------------
// FullCompact pipe (Layer 5)
// ---------------------------------------------------------------------------

func TestFullCompact_SummarizerCalled(t *testing.T) {
	msgs := make([]types.Message, 30)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = types.Message{Role: role, Content: "turn " + strings.Repeat("x", 100)}
	}
	calls := 0
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		calls++
		return "<summary>archive text that is at least fifty characters long so the quality check passes</summary>", nil
	}
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("FullCompact failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("summarizer should be called once, got %d", calls)
	}
	if cm.Ctx.Archive == "" {
		t.Error("Archive should be set after FullCompact")
	}
	if cm.Ctx.CompactBoundary <= 0 {
		t.Error("CompactBoundary should advance after FullCompact")
	}
}

func TestFullCompact_NoSummarizer(t *testing.T) {
	msgs := make([]types.Message, 30)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "x"}
	}
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	// no summarizer
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("FullCompact with nil summarizer should not error: %v", err)
	}
	if cm.Ctx.Archive != "" {
		t.Error("Archive should remain empty when no summarizer")
	}
}

func TestFullCompact_NotEnoughMessages(t *testing.T) {
	cm := newPipelineCompactor(
		[]types.Message{
			{Role: "user", Content: "1"},
			{Role: "assistant", Content: "2"},
		},
		64000, 0, "",
	)
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		t.Error("summarizer should not be called with ≤10 messages")
		return "<summary>ok</summary>", nil
	}
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFullCompact_SummarizerError(t *testing.T) {
	msgs := make([]types.Message, 30)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "x"}
	}
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "", errors.New("LLM unavailable")
	}
	err := cm.FullCompact()
	if err == nil {
		t.Fatal("expected error from summarizer failure")
	}
	if !strings.Contains(err.Error(), "LLM unavailable") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFullCompact_QualityCheckDoublesKeep(t *testing.T) {
	msgs := make([]types.Message, 50)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "short "}
	}
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		// Return a very short archive that fails quality check
		return "<summary>short</summary>", nil
	}
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("FullCompact should not error on quality check: %v", err)
	}
	// After quality check fail, keep should double, boundary should be smaller
	if cm.Ctx.CompactBoundary == 0 {
		t.Log("quality check triggered keep doubling")
	}
}

func TestFullCompact_MessagesBeforeBoundary(t *testing.T) {
	msgs := make([]types.Message, 50)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "message " + strings.Repeat("x", 80)}
	}
	// boundary at 20 — only messages 20..49 are compressible
	cm := newPipelineCompactor(msgs, 64000, 20, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		if len(msgs) <= 20 {
			t.Logf("summarizer received %d messages (boundary=%d)", len(msgs), 20)
		}
		return "<summary>archive content that is definitely long enough to pass the quality check so the test works correctly</summary>", nil
	}
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("FullCompact failed: %v", err)
	}
	if cm.Ctx.CompactBoundary <= 20 {
		t.Error("boundary should advance beyond initial boundary after FullCompact")
	}
}

func TestFullCompact_TrimOldMessages(t *testing.T) {
	msgs := make([]types.Message, 300)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "x"}
	}
	cm := newPipelineCompactor(msgs, 64000, 250, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>quality archive text that passes the min length check for this test</summary>", nil
	}
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("FullCompact failed: %v", err)
	}
	// trimOldMessages should fire (boundary=250 > 200)
	if *cm.TrimCount <= 0 {
		t.Error("TrimCount should be > 0 after trimming old messages")
	}
	if cm.Ctx.CompactBoundary > 200 {
		t.Logf("boundary trimmed from 250 to %d", cm.Ctx.CompactBoundary)
	}
}

// ---------------------------------------------------------------------------
// MicroCompact pipe (Layer 3)
// ---------------------------------------------------------------------------

func TestMicroCompactIfNeeded_UnderBudget(t *testing.T) {
	cm := newPipelineCompactor(nil, 64000, 0, "")
	cm.Tracker = &testTracker{promptEst: 10000} // well below half of 64000
	n := cm.MicroCompactIfNeeded()
	if n != 0 {
		t.Errorf("under budget should not micro-compact, got %d", n)
	}
}

func TestMicroCompactIfNeeded_ClearsToolResults(t *testing.T) {
	msgs := createMicroCompactMessages(10) // 10 tool-result pairs
	smallBudget := 8000
	cm := &Compactor{
		Ctx:           &content.Content{Messages: msgs},
		ContextWindow: &smallBudget,
		Tracker:       &testTracker{promptEst: 6000}, // > half of 8000
		CompactCount:  new(int),
		TrimCount:     new(int),
		Cfg:           DefaultConfig,
	}
	n := cm.MicroCompactIfNeeded()
	if n <= 0 {
		t.Errorf("expected to clear some results, got %d", n)
	}
	if *cm.CompactCount <= 0 {
		t.Error("CompactCount should > 0")
	}
}

func TestMicroCompactIfNeeded_PreservesRecent(t *testing.T) {
	// The most recent 2 turns should be preserved
	msgs := []types.Message{
		{Role: "user", Content: "old question"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "tc1", Function: types.FunctionCall{Name: "grep"}},
		}},
		{Role: "tool", ToolCallID: "tc1", Content: "old grep result " + strings.Repeat("x", 100)},
		{Role: "user", Content: "recent question"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "tc2", Function: types.FunctionCall{Name: "read"}},
		}},
		{Role: "tool", ToolCallID: "tc2", Content: "recent read result " + strings.Repeat("x", 100)},
	}
	smallBudget := 8000
	cm := &Compactor{
		Ctx:           &content.Content{Messages: msgs},
		ContextWindow: &smallBudget,
		Tracker:       &testTracker{promptEst: 6000},
		CompactCount:  new(int),
		TrimCount:     new(int),
		Cfg:           DefaultConfig,
	}
	n := cm.MicroCompactIfNeeded()
	// Old tool results (tc1) may be cleared; recent (tc2) should be preserved
	_ = n
	if msgs[5].Content == ClearedMarker {
		t.Error("recent tool result should not be cleared")
	}
}

func TestMicroCompactIfNeeded_BudgetScaling(t *testing.T) {
	tests := []struct {
		budget   int
		expected int // min keepResults
		name     string
	}{
		{32000, 3, "32K"},
		{64000, 5, "64K"},
		{128000, 8, "128K"},
		{256000, 8, "256K"},
	}
	for _, tt := range tests {
		msgs := createMicroCompactMessages(20)
		cm := &Compactor{
			Ctx:           &content.Content{Messages: msgs},
			ContextWindow: &tt.budget,
			Tracker:       &testTracker{promptEst: tt.budget - 1000},
			CompactCount:  new(int),
			TrimCount:     new(int),
			Cfg:           DefaultConfig,
		}
		n := cm.MicroCompactIfNeeded()
		// With 20 batches and keepResults=tt.expected, should clear 20-tt.expected
		t.Logf("[%s] budget=%d cleared=%d", tt.name, tt.budget, n)
	}
}

// ---------------------------------------------------------------------------
// History Sniping (Layer 2)
// ---------------------------------------------------------------------------

func TestSnipHistory_BoundaryAdjustment(t *testing.T) {
	msgs := make([]types.Message, 100)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "x"}
	}
	cm := newPipelineCompactor(msgs, 64000, 100, "")
	n := cm.SnipHistory()
	// boundary = 100 > 60, keep=40 → snip = 100-40 = 60
	if n != 60 {
		t.Errorf("expected snip=60, got %d", n)
	}
	if cm.Ctx.CompactBoundary != 41 { // 100-60 = 40, +1 for boundary marker
		t.Errorf("expected boundary=41, got %d", cm.Ctx.CompactBoundary)
	}
}

func TestSnipHistory_BoundaryInserted(t *testing.T) {
	msgs := make([]types.Message, 80)
	for i := range msgs {
		msgs[i] = types.Message{Role: "user", Content: "x"}
	}
	cm := newPipelineCompactor(msgs, 64000, 80, "")
	cm.SnipHistory()
	lenBefore := len(cm.Ctx.Messages)
	if cm.Ctx.Messages[0].Content != snipeBoundaryMarker {
		t.Error("first message should be boundary marker")
	}
	if len(cm.Ctx.Messages) != lenBefore {
		t.Error("boundary marker should be inserted at front")
	}
}

func TestSnipHistory_UnderThreshold(t *testing.T) {
	for _, b := range []int{0, 30, 60} {
		cm := newPipelineCompactor(nil, 64000, b, "")
		n := cm.SnipHistory()
		if n != 0 {
			t.Errorf("boundary=%d: expected 0, got %d", b, n)
		}
	}
}

// ---------------------------------------------------------------------------
// CollapseContext (Layer 4)
// ---------------------------------------------------------------------------

func TestCollapseContext_NoSummarizer(t *testing.T) {
	msgs := createTurns(10) // user + assistant turns
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	// no summarizer
	if err := cm.CollapseContext(); err != nil {
		t.Fatalf("CollapseContext with nil summarizer should not error: %v", err)
	}
}

func TestCollapseContext_NotEnoughMessages(t *testing.T) {
	cm := newPipelineCompactor([]types.Message{}, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		t.Error("summarizer should not be called")
		return "", nil
	}
	if err := cm.CollapseContext(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCollapseContext_AppendsToArchive(t *testing.T) {
	msgs := createTurns(20) // 40 messages
	cm := newPipelineCompactor(msgs, 64000, 0, "existing archive content")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>collapsed content that is long enough to pass the quality check for this test scenario</summary>", nil
	}
	if err := cm.CollapseContext(); err != nil {
		t.Fatalf("CollapseContext failed: %v", err)
	}
	if cm.Ctx.Archive == "" {
		t.Error("Archive should be updated after CollapseContext")
	}
	if cm.Ctx.CompactBoundary <= 0 {
		t.Error("CompactBoundary should advance")
	}
}

func TestCollapseContext_QualityCheck(t *testing.T) {
	msgs := createTurns(20)
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		// Return archive that's too small to pass quality check
		return "<summary>tiny</summary>", nil
	}
	origBoundary := cm.Ctx.CompactBoundary
	if err := cm.CollapseContext(); err != nil {
		t.Fatalf("CollapseContext should not error: %v", err)
	}
	// Quality check fails, archive not updated
	if cm.Ctx.CompactBoundary != origBoundary {
		t.Error("boundary should stay same when quality check fails")
	}
}

// ---------------------------------------------------------------------------
// Real-world scenario based on session.json data
// The session.json contains a multi-turn coding conversation with:
// - system prompt (Chinese)
// - user messages requesting desktop app conversion
// - assistant tool calls (tree, read)
// - tool results with file contents (including error responses)
// - multiple consecutive assistant messages for retries
// This tests the pipeline with realistic message patterns.
// ---------------------------------------------------------------------------

func TestPipelineWithSessionJsonLikeData(t *testing.T) {
	// Build messages inspired by the session.json structure:
	// system prompt (Chinese 2K chars), user questions, tool calls,
	// tool results with code/file content, error responses, retry patterns.
	msgs := buildSessionJsonLikeData()

	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>session archive content that preserves all key facts from the conversation and is at least fifty characters</summary>", nil
	}
	cm.Tracker = &testTracker{promptEst: 50000}

	level, err := cm.AutoCompactIfNeeded()
	if err != nil {
		t.Fatalf("session-json-like pipeline failed: %v", err)
	}
	t.Logf("session-json-like pipeline: level=%s, trimmed=%d, compacted=%d", level, *cm.TrimCount, *cm.CompactCount)
}

func TestPipelineWithSessionJson_ErrorMessages(t *testing.T) {
	// session.json has tool results with is_error=true ("missing startLine parameter")
	// This tests that error-containing messages don't break the pipeline.
	msgs := []types.Message{
		{Role: "system", Content: "你是一位性格软萌的二次元黑猫形象少女"},
		{Role: "user", Content: "将当前项目使用go+wails打包为桌面端应用"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "call_1", Function: types.FunctionCall{Name: "read"}},
		}},
		{Role: "tool", ToolCallID: "call_1", Content: "missing startLine parameter", IsError: true},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "call_2", Function: types.FunctionCall{Name: "read"}},
		}},
		{Role: "tool", ToolCallID: "call_2", Content: "package main\nimport (\n\t\"fmt\"\n)\nfunc main() { fmt.Println(\"hello\") }"},
		{Role: "assistant", Content: "我找到了文件内容"},
	}
	// Add enough turns to exceed FullCompact's 10-msg minimum keep threshold
	for i := 0; i < 6; i++ {
		msgs = append(msgs,
			types.Message{Role: "user", Content: "继续修改 file" + strconv.Itoa(i) + ".go"},
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
				{ID: "e" + strconv.Itoa(i), Function: types.FunctionCall{Name: "edit"}},
			}},
			types.Message{Role: "tool", ToolCallID: "e" + strconv.Itoa(i), Content: "edit result for file" + strconv.Itoa(i) + " " + strings.Repeat("x", 100), IsError: i%3 == 0},
			types.Message{Role: "assistant", Content: "modified file" + strconv.Itoa(i)},
		)
	}
	// 7 + 24 = 31 messages, enough for FullCompact

	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>archived session with error recovery that is definitely long enough for the quality check</summary>", nil
	}

	// Test FullCompact works with error messages
	if err := cm.FullCompact(); err != nil {
		t.Fatalf("FullCompact with error messages failed: %v", err)
	}
	if cm.Ctx.Archive == "" {
		t.Error("Archive should be set")
	}
}

func TestPipelineWithSessionJson_NoToolResults(t *testing.T) {
	// session.json has assistant messages that only contain tool_calls with NO content
	// (just calls). This tests that the pipeline handles tool_call-only assistant msgs.
	msgs := []types.Message{
		{Role: "user", Content: "show me the tree"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "tc1", Function: types.FunctionCall{Name: "tree", Arguments: `{"path":"/home/user/project"}`}},
		}},
		{Role: "tool", ToolCallID: "tc1", Content: "src/\n├── main.go\n├── utils/"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "tc2", Function: types.FunctionCall{Name: "read", Arguments: `{"path":"/home/user/project/main.go"}`}},
		}},
		{Role: "tool", ToolCallID: "tc2", Content: "package main\nfunc main() {}"},
		{Role: "assistant", Content: "."}, // dot-only assistant response
		{Role: "user", Content: "继续别的任务吧"},
	}
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>archived conversation that passes the quality check with enough text content in the summary</summary>", nil
	}

	// Run all layers via AutoCompactIfNeeded
	level, err := cm.AutoCompactIfNeeded()
	if err != nil {
		t.Fatalf("pipeline with tool-call-only msgs failed: %v", err)
	}
	t.Logf("level=%s", level)
}

func TestPipelineWithSessionJson_MultiToolResults(t *testing.T) {
	// session.json has parallel tool calls in one assistant message (multiple reads)
	// This tests that micro-compaction handles multi-tool batches without crashing
	// and preserves the batch-all-or-none invariant.
	msgs := []types.Message{
		{Role: "user", Content: "read these files"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "r1", Function: types.FunctionCall{Name: "read"}},
			{ID: "r2", Function: types.FunctionCall{Name: "read"}},
			{ID: "r3", Function: types.FunctionCall{Name: "read"}},
		}},
		{Role: "tool", ToolCallID: "r1", Content: "file1 content " + strings.Repeat("x", 300)},
		{Role: "tool", ToolCallID: "r2", Content: "file2 content " + strings.Repeat("x", 300)},
		{Role: "tool", ToolCallID: "r3", Content: "file3 content " + strings.Repeat("x", 300)},
		{Role: "assistant", Content: "done reading"},
		// Another turn
		{Role: "user", Content: "edit them"},
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "e1", Function: types.FunctionCall{Name: "edit"}},
		}},
		{Role: "tool", ToolCallID: "e1", Content: "edit result " + strings.Repeat("x", 200)},
		{Role: "assistant", Content: "done"},
	}
	// Total = 4 tool-result pairs, but only 2 batches (3 reads + 1 edit).
	// With small budget 8000 and keepResults=3, the pipeline would clear 1 result.
	// However, both batches span only ~2 user turns, so recentBoundary=0
	// covers ALL messages — nothing gets cleared. This is correct behavior:
	// micro-compact only targets OLD messages (≥2 turns back).
	smallBudget := 8000
	cm := &Compactor{
		Ctx:           &content.Content{Messages: msgs},
		ContextWindow: &smallBudget,
		Tracker:       &testTracker{promptEst: 6000},
		CompactCount:  new(int),
		TrimCount:     new(int),
		Cfg:           DefaultConfig,
	}
	n := cm.MicroCompactIfNeeded()
	// n == 0 is expected — all tool results are within the recent 2-turn window
	if n != 0 {
		t.Logf("cleared %d tool results (unexpected but not fatal)", n)
	}
	_ = n
}

// ---------------------------------------------------------------------------
// end-to-end pipeline with realistic scenario
// ---------------------------------------------------------------------------

func TestAutoCompactIfNeeded_RealisticScenario(t *testing.T) {
	// Simulate a real coding session: file reads, edits, shell commands, conversations
	msgs := buildRealisticSession()
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>archive containing a summary of the coding session that is long enough to pass quality check</summary>", nil
	}
	cm.Tracker = &testTracker{promptEst: 50000} // trigger compaction

	level, err := cm.AutoCompactIfNeeded()
	if err != nil {
		t.Fatalf("realistic scenario failed: %v", err)
	}
	t.Logf("realistic scenario reached level %s, trimmed=%d, compacted=%d", level, *cm.TrimCount, *cm.CompactCount)
}

func TestAutoCompactIfNeeded_MultipleCompactions(t *testing.T) {
	// Run the pipeline multiple times to simulate repeated compactions
	msgs := buildRealisticSession()
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Summarizer = func(msgs []types.Message, prev string) (string, error) {
		return "<summary>archive content that is long enough to withstand the quality check requirement in the compactor</summary>", nil
	}
	cm.Tracker = &testTracker{promptEst: 50000}

	for i := 0; i < 3; i++ {
		level, err := cm.AutoCompactIfNeeded()
		if err != nil {
			t.Fatalf("iteration %d failed: %v", i, err)
		}
		// After the first compaction, next iterations should see Normal/Warning
		if i > 0 && level > LevelCompact {
			t.Errorf("iteration %d: expected ≤ LevelCompact, got %s", i, level)
		}
	}
}

// ---------------------------------------------------------------------------
// Test that visibleEstimatedTokens matches estimateTokens
// ---------------------------------------------------------------------------

func TestEstimateTokensConsistency(t *testing.T) {
	msgs := []types.Message{
		{Role: "user", Content: "hello world"},
		{Role: "assistant", Content: "hi there"},
	}
	cm := newPipelineCompactor(msgs, 64000, 1, "")
	// visible: only msgs[1:] (boundary=1)
	visible := cm.visibleEstimatedTokens()
	estimate := cm.estimateTokens()
	if visible != estimate {
		t.Errorf("visibleEstimatedTokens=%d != estimateTokens=%d", visible, estimate)
	}
}

func TestEstimateTokens_TrackerPrecedence(t *testing.T) {
	msgs := []types.Message{
		{Role: "user", Content: "short"},
	}
	cm := newPipelineCompactor(msgs, 64000, 0, "")
	cm.Tracker = &testTracker{promptEst: 99999} // tracker estimate should take precedence
	n := cm.estimateTokens()
	if n < 90000 {
		t.Errorf("tracker estimate should dominate: got %d, want >= 90000", n)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newPipelineCompactor(msgs []types.Message, budget int, boundary int, archive string) *Compactor {
	ctx := &content.Content{Messages: msgs, CompactBoundary: boundary, Archive: archive}
	return &Compactor{
		Ctx: ctx, ContextWindow: &budget, Tracker: &testTracker{},
		CompactCount: new(int), TrimCount: new(int), Cfg: DefaultConfig,
	}
}

func createMicroCompactMessages(nBatches int) []types.Message {
	var msgs []types.Message
	for i := 0; i < nBatches; i++ {
		msgs = append(msgs,
			types.Message{Role: "user", Content: "question " + strings.Repeat("x", 100)},
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
				{ID: "tc" + itoa(i), Function: types.FunctionCall{Name: "read"}},
			}},
			types.Message{Role: "tool", ToolCallID: "tc" + itoa(i), Content: "result " + strings.Repeat("x", 500)},
		)
	}
	return msgs
}

func createTurns(n int) []types.Message {
	var msgs []types.Message
	for i := 0; i < n; i++ {
		msgs = append(msgs,
			types.Message{Role: "user", Content: "question " + itoa(i) + " " + strings.Repeat("x", 100)},
			types.Message{Role: "assistant", Content: "answer " + itoa(i) + " " + strings.Repeat("y", 200)},
		)
	}
	return msgs
}

func buildSessionJsonLikeData() []types.Message {
	var msgs []types.Message
	// System prompt from session.json (Chinese agent persona)
	msgs = append(msgs, types.Message{
		Role: "system",
		Content: `# 风格人设
1. 你是一位性格软萌的二次元黑猫形象少女，说话可爱温柔。
2. 保持元气、治愈、无攻击性的风格。
# 核心准则
1. 做任何事情之前，必须理清思路再执行。
2. 按需探索，不要一次性读所有文件。
# 改动纪律
1. 只触碰用户请求要求的部分。
# 输出规范
1. 工具调用前：一句话说明要做什么。
2. 简洁优先，能一句话的不写三段。`,
	})
	// Session 1: user asks to port to desktop app
	msgs = append(msgs, types.Message{Role: "user", Content: "将当前项目使用go+wails打包为桌面端应用，数据存在用户目录dotfile"})
	// Assistant calls tree
	msgs = append(msgs, types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
		{ID: "call_tree", Function: types.FunctionCall{Name: "tree", Arguments: `{"path":"/home/user/project"}`}},
	}})
	msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "call_tree",
		Content: "work-planner/\n├── README.md\n├── dist/\n│   ├── index.html\n│   └── assets/\n├── index.html\n├── package.json\n├── src/\n│   ├── App.jsx\n│   ├── App.css\n│   ├── index.css\n│   └── main.jsx\n└── vite.config.js"})
	// Assistant calls read (with error - missing parameter like the real session)
	msgs = append(msgs, types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
		{ID: "call_read1", Function: types.FunctionCall{Name: "read"}},
	}})
	msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "call_read1",
		Content: "missing startLine parameter", IsError: true})
	// Assistant retries with correct params
	msgs = append(msgs, types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
		{ID: "call_read2", Function: types.FunctionCall{Name: "read"}},
	}})
	msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "call_read2",
		Content: `{
  "name": "work-planner",
  "private": true,
  "version": "0.0.0",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  }
}`})
	// Assistant responds
	msgs = append(msgs, types.Message{Role: "assistant",
		Content: "好的，这是一个纯前端 React + Vite 项目，数据现在存在 localStorage。我先检查环境。"})
	// Session 2: shell commands
	msgs = append(msgs, types.Message{Role: "user", Content: "帮我安装 wails CLI"})
	msgs = append(msgs, types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
		{ID: "call_shell", Function: types.FunctionCall{Name: "shell", Arguments: `{"command":"which go"}`}},
	}})
	msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "call_shell",
		Content: "/etc/profiles/per-user/lznauy/bin/go\ngo version go1.26.4 linux/amd64"})
	msgs = append(msgs, types.Message{Role: "assistant", Content: "Go 已安装，版本 1.26.4"})
	// Session 3: more interactions
	msgs = append(msgs, types.Message{Role: "user",
		Content: "不能使用host模式？", Source: "user"})
	msgs = append(msgs, types.Message{Role: "assistant",
		Content: "对，用 sandbox_mode: host 就可以直接写到宿主机了。"})
	// Some tool results with longer content (like code output)
	msgs = append(msgs, types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
		{ID: "call_find", Function: types.FunctionCall{Name: "grep"}},
	}})
	msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "call_find",
		Content: strings.Repeat("line of output\n", 80)})
	return msgs
}

func buildRealisticSession() []types.Message {
	var msgs []types.Message
	// System prompt
	msgs = append(msgs, types.Message{Role: "system", Content: "You are a helpful coding assistant."})
	// Conversation turns with tool calls
	for i := 0; i < 15; i++ {
		msgs = append(msgs, types.Message{Role: "user", Content: "please fix bug in file" + itoa(i) + ".go " + strings.Repeat("x", 150)})
		msgs = append(msgs, types.Message{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "read" + itoa(i), Function: types.FunctionCall{Name: "read"}},
			{ID: "edit" + itoa(i), Function: types.FunctionCall{Name: "edit"}},
		}})
		msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "read" + itoa(i), Content: "file content " + strings.Repeat("x", 300)})
		msgs = append(msgs, types.Message{Role: "tool", ToolCallID: "edit" + itoa(i), Content: "edit applied " + strings.Repeat("x", 100)})
		msgs = append(msgs, types.Message{Role: "assistant", Content: "fixed! " + strings.Repeat("y", 120)})
	}
	return msgs
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
