package provider

import (
	"nekocode/bot/provider/anthropic"
	"nekocode/bot/provider/openai"
)

// NewClientWithProtocol creates an LLM client with explicit protocol selection.
// protocol: "openai" or "anthropic".
func NewClientWithProtocol(apiKey, baseURL, model, protocol string) LLM {
	switch protocol {
	case "anthropic":
		return anthropic.New(apiKey, baseURL, model)
	default:
		c := openai.New(apiKey, baseURL, model)
		c.SetDisableThinking(true)
		return c
	}
}
