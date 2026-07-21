package view

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDisplayMessageJSONUsesClientFieldNames(t *testing.T) {
	data, err := json.Marshal(DisplayMessage{
		Role:    "assistant",
		Content: "done",
		Blocks: []DisplayBlock{{
			ToolName: "shell",
			Args:     `{"command":"go test"}`,
			Content:  "ok",
			IsError:  true,
		}},
		Images: []ImageRef{{
			Path:   "/tmp/image.png",
			URL:    "https://example.test/image.png",
			Width:  640,
			Height: 480,
		}},
	})
	if err != nil {
		t.Fatalf("marshal display message: %v", err)
	}
	got := string(data)
	for _, want := range []string{`"role"`, `"content"`, `"blocks"`, `"toolName"`, `"isError"`, `"images"`, `"path"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("display message json missing %s: %s", want, got)
		}
	}
	for _, unwanted := range []string{`"Role"`, `"Content"`, `"Blocks"`, `"ToolName"`, `"IsError"`, `"Images"`, `"Path"`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("display message json leaked Go field %s: %s", unwanted, got)
		}
	}
}

func TestBotStatsJSONUsesClientFieldNames(t *testing.T) {
	data, err := json.Marshal(BotStats{PromptTokens: 10, CompletionTokens: 5, Duration: "1s"})
	if err != nil {
		t.Fatalf("marshal bot stats: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"promptTokens"`) || !strings.Contains(got, `"completionTokens"`) {
		t.Fatalf("bot stats json missing client names: %s", got)
	}
	if strings.Contains(got, `"PromptTokens"`) || strings.Contains(got, `"CompletionTokens"`) {
		t.Fatalf("bot stats json leaked Go fields: %s", got)
	}
}
