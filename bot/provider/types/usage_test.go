package types

import (
	"encoding/json"
	"testing"
)

func TestStreamUsage_OpenAI_Mimo_DeepSeek(t *testing.T) {
	// All OpenAI-compatible APIs use prompt_tokens_details.cached_tokens.
	// DeepSeek additionally sends flat fields; the nested field wins.
	data := `{"prompt_tokens":100,"completion_tokens":50,"prompt_tokens_details":{"cached_tokens":80}}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}

	u.Normalize()

	if u.CacheHitTokens != 80 {
		t.Errorf("CacheHitTokens = %d, want 80", u.CacheHitTokens)
	}
	if u.CacheMissTokens != 20 {
		t.Errorf("CacheMissTokens = %d, want 20", u.CacheMissTokens)
	}
}

func TestStreamUsage_Anthropic(t *testing.T) {
	// Anthropic client builds StreamUsage directly from cache_read_input_tokens.
	u := &StreamUsage{PromptTokens: 100, CacheHitTokens: 80}
	u.Normalize()

	if u.CacheHitTokens != 80 {
		t.Errorf("CacheHitTokens = %d, want 80", u.CacheHitTokens)
	}
	if u.CacheMissTokens != 20 {
		t.Errorf("CacheMissTokens = %d, want 20", u.CacheMissTokens)
	}
}

func TestStreamUsage_NoCacheDetails(t *testing.T) {
	// API returns usage without prompt_tokens_details.
	data := `{"prompt_tokens":100,"completion_tokens":50}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}

	u.Normalize()

	if u.CacheHitTokens != 0 {
		t.Errorf("CacheHitTokens = %d, want 0", u.CacheHitTokens)
	}
	if u.CacheMissTokens != 0 {
		t.Errorf("CacheMissTokens = %d, want 0", u.CacheMissTokens)
	}
}

func TestStreamUsage_OpenAIZeroCachedTokensIsFullMiss(t *testing.T) {
	data := `{"prompt_tokens":100,"completion_tokens":50,"prompt_tokens_details":{"cached_tokens":0}}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}
	u.Normalize()
	if u.CacheHitTokens != 0 || u.CacheMissTokens != 100 {
		t.Fatalf("cache usage = hit %d / miss %d, want 0/100", u.CacheHitTokens, u.CacheMissTokens)
	}
}

func TestStreamUsage_DeepSeekFlatFieldsFallback(t *testing.T) {
	// DeepSeek's flat fields populate CacheHit/Miss via Normalize when the
	// standard nested field is absent (e.g. older/proxied responses).
	data := `{"prompt_tokens":100,"completion_tokens":50,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}

	u.Normalize()

	if u.CacheHitTokens != 80 {
		t.Errorf("CacheHitTokens = %d, want 80 (from flat field)", u.CacheHitTokens)
	}
	if u.CacheMissTokens != 20 {
		t.Errorf("CacheMissTokens = %d, want 20 (from flat field)", u.CacheMissTokens)
	}
}

func TestStreamUsage_NestedFieldWinsOverFlat(t *testing.T) {
	// When both forms are present, the OpenAI-standard nested field wins.
	data := `{"prompt_tokens":100,"completion_tokens":50,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20,"prompt_tokens_details":{"cached_tokens":80}}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}

	if u.CacheHitTokens != 0 {
		t.Errorf("CacheHitTokens before Normalize = %d, want 0 (filled by Normalize)", u.CacheHitTokens)
	}

	u.Normalize()

	if u.CacheHitTokens != 80 {
		t.Errorf("CacheHitTokens = %d, want 80 (from standard field)", u.CacheHitTokens)
	}
	if u.CacheMissTokens != 20 {
		t.Errorf("CacheMissTokens = %d, want 20", u.CacheMissTokens)
	}
}

func TestStreamUsage_ZeroPrompt(t *testing.T) {
	data := `{"prompt_tokens":0,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":5}}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}

	u.Normalize()

	if u.CacheHitTokens != 5 {
		t.Errorf("CacheHitTokens = %d, want 5", u.CacheHitTokens)
	}
	if u.CacheMissTokens != 0 {
		t.Errorf("CacheMissTokens = %d, want 0 (prompt=0 guard)", u.CacheMissTokens)
	}
}

func TestStreamUsage_FlatMissWinsOverArithmetic(t *testing.T) {
	// A provider-reported miss is trusted over prompt-hit arithmetic —
	// the two are equal today, but the reported value is the guarantee.
	data := `{"prompt_tokens":100,"completion_tokens":50,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":15}`
	var u StreamUsage
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}
	u.Normalize()
	if u.CacheMissTokens != 15 {
		t.Errorf("CacheMissTokens = %d, want 15 (reported flat value, not 100-80)", u.CacheMissTokens)
	}
}
