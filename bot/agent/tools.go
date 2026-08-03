package agent

import (
	"fmt"
	"maps"
	"os"
	"slices"

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

	cleanupSubagents, deniedSubagents, kept := r.prepareSubagentCallbacks(filtered.Allowed, filtered.AllowedIdx, callback)
	filtered.Allowed = kept
	maps.Copy(filtered.Blocked, deniedSubagents)
	defer cleanupSubagents()

	r.agent.deps.toolExecutor.PreparePreviewsContext(r.agent.getCtx(), filtered.Allowed)
	emitStartCallbacks(calls, filtered.Blocked, callback)

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
	return slices.ContainsFunc(results, r.agent.applyPostToolHookResult)
}

type filteredCalls struct {
	Allowed      []core.ToolCallItem
	AllowedIdx   []int // original index in the caller's calls slice, parallel to Allowed
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
		out.AllowedIdx = append(out.AllowedIdx, i)
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

var subSlotFullReason = fmt.Sprintf("task not started: subagent slot pool is full (%d/%d); retry after the current batch finishes", maxSubSlots, maxSubSlots)

// prepareSubagentCallbacks reserves a sub-agent slot for each "task" call and
// wires per-sub-agent event forwarding. When the slot pool is full (more than
// maxSubSlots tasks in one batch) overflow tasks are denied fast — removed from
// the returned allowed slice and reported in the denied map keyed by original
// call index — instead of blocking the main goroutine until the batch finishes.
func (r *toolRunner) prepareSubagentCallbacks(allowed []core.ToolCallItem, allowedIdx []int, callback RunCallback) (func(), map[int]string, []core.ToolCallItem) {
	denied := make(map[int]string)
	kept := make([]core.ToolCallItem, 0, len(allowed))
	var taskInfos []subSlotInfo
	for i, c := range allowed {
		if c.Name != "task" {
			kept = append(kept, c)
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
			denied[allowedIdx[i]] = subSlotFullReason
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
		c.Args["_sub_callback"] = taskbridge.TaskCallbackFn(func(ev protocol.StepEvent) {
			if callback == nil {
				return
			}
			ev.SubAgentID = sid
			ev.SubAgentColor = cid
			callback(ev)
		})
		kept = append(kept, c)
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
	}, denied, kept
}
