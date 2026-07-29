// Command minimal-agent shows the smallest useful assembly of the
// bot/agent/runtime public contract: a context manager, a tool registry
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

	"nekocode/bot/agent/runtime"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/provider"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/catalog"
	commonview "nekocode/common/view"
)

func main() {
	apiKey := os.Getenv("NEKOCODE_API_KEY")
	if apiKey == "" {
		fmt.Println("set NEKOCODE_API_KEY to run this example")
		return
	}
	model := getenv("NEKOCODE_MODEL", "gpt-4o-mini")

	llm := provider.NewClientWithProtocol(apiKey, os.Getenv("NEKOCODE_BASE_URL"), model, getenv("NEKOCODE_PROTOCOL", "openai"))

	registry := tools.NewRegistry()
	catalog.RegisterAll(registry, nil)

	agent := runtime.New(context.Background(), runtime.AgentConfig{
		CtxMgr:   ctxmgr.NewSub("You are a helpful assistant.", 128000, nil),
		LLM:      llm,
		Registry: registry,
	})

	result := agent.Run("你好，介绍一下你自己", func(ev commonview.StepEvent) {
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
