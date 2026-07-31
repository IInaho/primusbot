// Command minimal-agent shows the smallest useful assembly of the
// bot/agent public contract: a context manager, a tool registry
// with the builtin catalog, an LLM client, and one Run with step events
// printed to stdout.
//
// Required environment:
//
//	NEKOCODE_API_KEY   API key for the provider (required)
//	NEKOCODE_BASE_URL  provider base URL (optional)
//	NEKOCODE_MODEL     model name (optional, default gpt-4o-mini)
//	NEKOCODE_PROTOCOL  wire protocol (optional, default openai)
package main

import (
	"context"
	"fmt"
	"os"

	agentcore "nekocode/bot/agent"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/provider"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/catalog"
	"nekocode/protocol"
)

func main() {
	apiKey := os.Getenv("NEKOCODE_API_KEY")
	if apiKey == "" {
		fmt.Println("set NEKOCODE_API_KEY to run this example")
		return
	}
	model := getenv("NEKOCODE_MODEL", "gpt-4o-mini")

	llm := provider.New(provider.Config{
		APIKey: apiKey, BaseURL: os.Getenv("NEKOCODE_BASE_URL"),
		Model: model, Protocol: getenv("NEKOCODE_PROTOCOL", "openai"),
	})

	registry := tools.New()
	catalog.RegisterAll(registry, nil)

	agent := agentcore.New(context.Background(), agentcore.Config{
		Context: ctxmgr.New(ctxmgr.Config{
			SystemPrompt: "You are a helpful assistant.", ContextWindow: 128000,
		}),
		Model: llm,
		Tools: registry,
	})

	result := agent.Run("你好，介绍一下你自己", func(ev protocol.StepEvent) {
		fmt.Printf("[%s] %s %s\n", ev.Action, ev.ToolName, ev.Output)
	})
	fmt.Println("---")
	fmt.Println(result.FinalOutput)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
