package runtime

import (
	"fmt"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/taskbridge"
	"nekocode/common/debug"
	commonview "nekocode/common/view"

	"github.com/google/uuid"
)

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
			debug.Log("subSlotMgr: Acquire failed for %s (all slots full)", subType)
			continue
		}
		if callback != nil {
			callback(commonview.StepEvent{Action: commonview.StepActionSubAgentStart, ToolName: subType, ToolArgs: subID, Output: fmt.Sprint(colorIdx)})
		}
		sid := subID
		cid := colorIdx
		taskInfos = append(taskInfos, subSlotInfo{sid, cid})
		allowed[i].Args["_sub_callback"] = taskbridge.TaskCallbackFn(func(ev commonview.StepEvent) {
			if callback == nil {
				return
			}
			sidTag := fmt.Sprintf("%s:%d", sid, cid)
			switch ev.Action {
			case commonview.StepActionSubToolStart:
				ev.Output = sidTag
			case commonview.StepActionSubExecuteTool:
				ev.ToolArgs = sidTag
			}
			callback(ev)
		})
	}

	return func() {
		for _, ti := range taskInfos {
			r.agent.deps.subSlotMgr.Release(ti.subID)
			if callback != nil {
				callback(commonview.StepEvent{Action: commonview.StepActionSubAgentEnd, ToolArgs: ti.subID})
			}
		}
	}
}
