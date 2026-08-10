package subagent

import (
	"testing"

	"nekocode/bot/extension/tool"
)

func TestNewWiresEngineDependencies(t *testing.T) {
	registry := tools.New()
	engine := New(Config{Tools: registry})

	if engine == nil || engine.toolRegistry != registry {
		t.Fatal("New returned an engine without its tool registry")
	}
}

func TestBuildResult(t *testing.T) {
	r := buildResult("hello world", runMeta{totalTokens: 100, toolUseCount: 3, durationMs: 500})
	if r.Status != StatusCompleted {
		t.Error("should be StatusCompleted")
	}
	if r.Content != "hello world" {
		t.Errorf("Content = %q", r.Content)
	}
}

func TestCompletedResultDoesNotInferSafetyFromText(t *testing.T) {
	content := "The log never stores credentials or .env data; rm -rf is only a quoted example."
	r := buildResult(content, runMeta{})

	if got := FormatResult(r); got != content {
		t.Fatalf("FormatResult = %q, want unchanged content", got)
	}
}

func TestBuildPartialResult(t *testing.T) {
	r := buildPartialResult("partial data", runMeta{})
	if r.Status != StatusPartial || r.Content != "partial data" {
		t.Error("wrong partial result")
	}
}

func TestBuildFailedResult(t *testing.T) {
	r := buildFailedResult("connection refused", runMeta{})
	if r.Status != StatusFailed || r.Content != "connection refused" {
		t.Error("wrong failed result")
	}
}

func TestFormatResult_Normal(t *testing.T) {
	r := &Result{Content: "task done"}
	if s := FormatResult(r); s != "task done" {
		t.Errorf("FormatResult = %q, want %q", s, "task done")
	}
}
