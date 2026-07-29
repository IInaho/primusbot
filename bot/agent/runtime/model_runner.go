package runtime

import (
	"context"
	"strings"

	"nekocode/bot/agent/llmstream"
	"nekocode/bot/policy"
	"nekocode/bot/provider/types"
	"nekocode/bot/view"
	"nekocode/common/debug"

	"nekocode/bot/tools/runtime/core"
)

const synthesizePrompt = "Based on the information collected above, provide a final answer. Do NOT call any more tools. Output your conclusion directly."
const fallbackSynthesize = "Unable to produce a final summary — the model is currently unavailable. The task's tool operations may have completed, but the results could not be synthesized. Please try again or check the conversation log for details."

// modelRunner wraps the LLM call for a turn: streaming, retry, response
// classification, and final-answer synthesis.
type modelRunner struct {
	agent *Agent
}

func newModelRunner(agent *Agent) *modelRunner {
	return &modelRunner{agent: agent}
}

func (r *modelRunner) reason(input string) *reasoningResult {
	a := r.agent
	a.stream.emitPhase(view.PhaseThinking)
	if strings.HasPrefix(input, "/") && !strings.Contains(input, " ") {
		return commandResult()
	}

	toolCalls, textContent, err := r.callLLMForTool()
	return fromLLM(toolCalls, textContent, err)
}

func (r *modelRunner) callLLMForTool() ([]core.ToolCallItem, string, error) {
	a := r.agent
	toolDefs := core.ToToolDefs(a.deps.toolRegistry.Descriptors())
	messages := a.deps.ctxMgr.Build()
	messages = r.applyPreModelRequestHooks(messages)

	result, err := llmstream.CallLLMWithRetry(a.getCtx(), a.deps.llmClient, func() llmstream.LLMCallOptions {
		return llmstream.LLMCallOptions{
			Ctx:       a.getCtx(),
			Messages:  messages,
			ToolDefs:  toolDefs,
			Callbacks: r.streamCallbacks(),
			CheckDone: a.life.Finished().Load,
		}
	})
	if err != nil {
		return nil, "", err
	}

	textContent := result.Text
	if result.Reasoning != "" {
		a.stream.lastReason = result.Reasoning
	}
	if len(result.ToolCalls) > 0 {
		a.deps.ctxMgr.AddAssistantToolCall(textContent, a.stream.lastReason, llmstream.ToLLMToolCalls(result.ToolCalls))
	}
	return result.ToolCalls, textContent, nil
}

// synthesize produces a final answer when the run ended without one (step
// limit, repeated failures): it asks the LLM once for a tool-free summary
// and persists it. On failure or garbled output it falls back to a fixed
// message — no retries: this path fires precisely when the model is already
// misbehaving, so extra calls rarely help.
func (r *modelRunner) synthesize() string {
	a := r.agent
	text, err := r.streamSynthesize(a.getCtx())
	if err != nil || !usableFinalText(text) {
		debug.Log("synthesize: LLM summary failed (err=%v), using fallback", err)
		text = fallbackSynthesize
	}
	a.deps.ctxMgr.AddAssistantResponse(text, "")
	return text
}

// usableFinalText reports whether text is a non-empty, non-garbled final answer.
func usableFinalText(text string) bool {
	return text != "" && !isGarbledToolCall(text)
}

func (r *modelRunner) streamSynthesize(ctx context.Context) (string, error) {
	a := r.agent
	messages := a.deps.ctxMgr.Build()
	messages = append(messages, types.Message{Role: "user", Content: synthesizePrompt})

	result, err := llmstream.CallLLM(a.deps.llmClient, llmstream.LLMCallOptions{
		Ctx:       ctx,
		Messages:  messages,
		Callbacks: r.streamCallbacks(),
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (r *modelRunner) applyPreModelRequestHooks(messages []types.Message) []types.Message {
	gov := r.agent.deps.gov
	if gov == nil {
		return messages
	}
	hints := collectHints(gov.BeforeModel(countToolResults(messages)))
	if len(hints) == 0 {
		return messages
	}
	return append(messages, types.Message{
		Role:    "system",
		Content: policy.FormatHints(hints),
		Source:  "hook",
	})
}

func countToolResults(messages []types.Message) int {
	var count int
	for _, m := range messages {
		if m.Role == "tool" {
			count++
		}
	}
	return count
}

func (r *modelRunner) streamCallbacks() llmstream.StreamCallbacks {
	a := r.agent
	return llmstream.StreamCallbacks{
		OnText: func(delta string) {
			a.stream.emitText(delta)
		},
		OnReasoning: func(delta string) {
			a.stream.emitReasoning(delta)
		},
		OnPhase: func(phase string) {
			a.stream.emitPhase(phase)
		},
		AddTokens: func(prompt, completion int) {
			a.AddTokens(prompt, completion)
		},
		RecordUsage: func(prompt, completion int) {
			a.deps.ctxMgr.RecordUsage(prompt, completion)
		},
		RecordCache: func(hit, miss int) {
			a.deps.ctxMgr.RecordCache(hit, miss)
		},
	}
}
