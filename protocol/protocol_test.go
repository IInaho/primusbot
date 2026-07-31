package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMetricsJSONUsesProtocolFieldNames(t *testing.T) {
	data, err := json.Marshal(Metrics{PromptTokens: 10, CompletionTokens: 5, Duration: "1s"})
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"promptTokens"`) || !strings.Contains(got, `"completionTokens"`) {
		t.Fatalf("metrics json missing protocol names: %s", got)
	}
	if strings.Contains(got, `"PromptTokens"`) || strings.Contains(got, `"CompletionTokens"`) {
		t.Fatalf("metrics json leaked Go fields: %s", got)
	}
}
