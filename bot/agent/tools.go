package agent

import (
	"os"

	"github.com/google/uuid"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/policy"
	"nekocode/bot/provider/types"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/taskbridge"
	"nekocode/bot/tools/runtime/toolutil"
	"nekocode/logger"
	"nekocode/protocol"
)

// toolRunner orchestrates tool execution within a turn: filtering, execution,
// result feedback, and post-tool hooks.
type toolRunner struct {
	agent *Agent
}

func newToolRunner(agent *Agent) *toolRunner {
	return &toolRunner{agent: agent}
}

func (r *toolRunner) executeAndFeedback(calls []core.ToolCallItem, textContent string, callback RunCallback) bool {
	if textContent != "" && callback != nil {
		callback(protocol.StepEvent{Action: protocol.StepActionThink, Output: textContent})
	}

	filtered := r.filterToolCalls(calls)
	r.agent.deps.toolExecutor.PreparePreviewsContext(r.agent.getCtx(), filtered.Allowed)
	emitStartCallbacks(calls, filtered.Blocked, callback)

	cleanupSubagents := r.prepareSubagentCallbacks(filtered.Allowed, callback)
	defer cleanupSubagents()

	execResults := r.executeAllowedTools(filtered.Allowed, callback)
	results := mergeResults(calls, filtered.Blocked, execResults)
	policyResults := r.recordToolResults(calls, filtered.Blocked, results)

	msgs := emitResultCallbacks(calls, filtered.Blocked, results, callback)
	r.addToolResultsAndHints(calls, msgs, filtered.PreToolHints)

	if r.applyPolicyResults(policyResults) {
		return true
	}
	r.agent.run.step++
	return false
}

func (r *toolRunner) applyPolicyResults(results []policy.Result) bool {
	for _, result := range results {
		if r.agent.applyPostToolHookResult(result) {
			return true
		}
	}
	return false
}

type filteredCalls struct {
	Allowed      []core.ToolCallItem
	Blocked      map[int]string
	PreToolHints []*policy.Hint
}

const policyBlockedDefault = "blocked by policy"

func (r *toolRunner) filterToolCalls(calls []core.ToolCallItem) filteredCalls {
	out := filteredCalls{
		Allowed: make([]core.ToolCallItem, 0, len(calls)),
		Blocked: make(map[int]string),
	}
	for i, c := range calls {
		if r.applyPreToolPolicy(c, out.Blocked, i, &out.PreToolHints) {
			continue
		}

		out.Allowed = append(out.Allowed, c)
	}
	return out
}

func (r *toolRunner) applyPreToolPolicy(c core.ToolCallItem, blocked map[int]string, idx int, hints *[]*policy.Hint) bool {
	gov := r.agent.deps.gov
	if gov == nil {
		return false
	}
	shouldBlock := false
	request := policy.ToolRequest{
		Name:         c.Name,
		Args:         c.Args,
		TargetExists: toolTargetExists(c),
	}
	for _, result := range gov.BeforeTool(request) {
		if result.Hint != nil {
			*hints = append(*hints, result.Hint)
		}
		if result.BlockTool != nil && (result.BlockTool.Tool == "" || result.BlockTool.Tool == c.Name) {
			blocked[idx] = result.BlockTool.Reason
			if blocked[idx] == "" {
				blocked[idx] = policyBlockedDefault
			}
			shouldBlock = true
		}
		if result.Stop != nil {
			blocked[idx] = policyBlockedStop(result.Stop.String())
			shouldBlock = true
		}
	}
	return shouldBlock
}

func policyBlockedStop(stop string) string {
	return "blocked by stop policy: " + stop
}

func toolTargetExists(call core.ToolCallItem) bool {
	if call.Name != "write" && call.Name != "edit" {
		return false
	}
	targetPath, _ := call.Args["path"].(string)
	if targetPath != "" {
		if resolved, err := toolutil.ValidatePath(targetPath); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				return true
			}
		}
	}
	return false
}

func emitStartCallbacks(calls []core.ToolCallItem, blocked map[int]string, callback RunCallback) {
	if callback == nil {
		return
	}
	for i, c := range calls {
		action := protocol.StepActionToolStart
		preview, _ := c.Args["_preview"].(string)
		if reason, ok := blocked[i]; ok {
			action = protocol.StepActionToolBlocked
			preview = reason
		}
		callback(protocol.StepEvent{Action: action, CallID: c.ID, ToolName: c.Name, ToolArgs: core.FormatArgs(c.Args), Output: preview})
	}
}

func mergeResults(calls []core.ToolCallItem, blocked map[int]string, execResults []core.ToolCallResult) []core.ToolCallResult {
	results := make([]core.ToolCallResult, len(calls))
	execIdx := 0
	for i := range calls {
		if msg, ok := blocked[i]; ok {
			results[i] = core.ToolCallResult{ID: calls[i].ID, Name: calls[i].Name, Error: msg}
			continue
		}
		results[i] = execResults[execIdx]
		execIdx++
	}
	return results
}

func emitResultCallbacks(calls []core.ToolCallItem, blocked map[int]string, results []core.ToolCallResult, callback RunCallback) []types.Message {
	msgs := make([]types.Message, len(results))
	for i, r := range results {
		content := r.EffectiveOutput()
		msgs[i] = types.Message{Content: content, ToolCallID: r.ID, IsError: r.Error != ""}
		if callback != nil {
			if _, isBlocked := blocked[i]; isBlocked {
				continue
			}
			callback(protocol.StepEvent{Action: protocol.StepActionExecuteTool, CallID: r.ID, ToolName: r.Name, ToolArgs: core.FormatArgs(calls[i].Args), Output: content, IsError: r.Error != ""})
		}
	}
	return msgs
}

func (r *toolRunner) executeAllowedTools(allowed []core.ToolCallItem, callback RunCallback) []core.ToolCallResult {
	executor := r.agent.deps.toolExecutor
	if callback != nil {
		executor.SetPreviewFn(func(toolName string, _ map[string]any, preview string) {
			callback(protocol.StepEvent{Action: protocol.StepActionToolPreview, ToolName: toolName, Output: preview})
		})
	} else {
		executor.SetPreviewFn(nil)
	}
	if len(allowed) == 0 {
		return nil
	}
	return executor.ExecuteBatch(r.agent.getCtx(), allowed)
}

func (r *toolRunner) recordToolResults(calls []core.ToolCallItem, blocked map[int]string, results []core.ToolCallResult) []policy.Result {
	gov := r.agent.deps.gov
	if gov == nil {
		return nil
	}
	events := make([]policy.ToolResult, 0, len(calls))
	for i, tc := range calls {
		if msg, ok := blocked[i]; ok {
			events = append(events, policy.ToolResult{
				Name:        tc.Name,
				Args:        tc.Args,
				Blocked:     true,
				BlockReason: msg,
			})
			continue
		}
		events = append(events, policy.ToolResult{
			Name:   tc.Name,
			Args:   tc.Args,
			Output: results[i].Output,
			Error:  results[i].Error,
		})
	}
	return gov.RecordTools(events)
}

func (r *toolRunner) addToolResultsAndHints(calls []core.ToolCallItem, msgs []types.Message, preToolHints []*policy.Hint) {
	toolResults := make([]ctxmgr.ToolResultMsg, len(msgs))
	for i, m := range msgs {
		toolResults[i] = ctxmgr.ToolResultMsg{Message: m, ToolName: calls[i].Name}
	}
	r.agent.deps.ctxMgr.AddToolResultsBatch(toolResults)

	for _, h := range preToolHints {
		r.agent.injectHint(h)
	}
}

type subSlotInfo struct {
	subID    string
	colorIdx int
}

func (r *toolRunner) prepareSubagentCallbacks(allowed []core.ToolCallItem, callback RunCallback) func() {
	var taskInfos []subSlotInfo
	for i, c := range allowed {
		if c.Name != "task" {
			continue
		}
		subType, _ := c.Args["type"].(string)
		if subType == "" {
			subType = "executor"
		}
		subID := uuid.New().String()
		colorIdx, ok := r.agent.deps.subSlotMgr.Acquire(subID, subType)
		if !ok {
			logger.Log("subSlotMgr: Acquire failed for %s (all slots full)", subType)
			continue
		}
		if callback != nil {
			callback(protocol.StepEvent{
				Action:     protocol.StepActionSubAgentStart,
				SubAgentID: subID, SubAgentType: subType, SubAgentColor: colorIdx,
			})
		}
		sid := subID
		cid := colorIdx
		taskInfos = append(taskInfos, subSlotInfo{sid, cid})
		allowed[i].Args["_sub_callback"] = taskbridge.TaskCallbackFn(func(ev protocol.StepEvent) {
			if callback == nil {
				return
			}
			ev.SubAgentID = sid
			ev.SubAgentColor = cid
			callback(ev)
		})
	}

	return func() {
		for _, ti := range taskInfos {
			r.agent.deps.subSlotMgr.Release(ti.subID)
			if callback != nil {
				callback(protocol.StepEvent{
					Action:     protocol.StepActionSubAgentEnd,
					SubAgentID: ti.subID,
				})
			}
		}
	}
}
