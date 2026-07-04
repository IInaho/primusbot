package builtin

import (
	"fmt"
	"strings"

	"nekocode/bot/hooks"
)

const (
	toolResultThreshold = 40
	toolResultInterval  = 10
)

func ToolResultGuardrailHook() hooks.Hook {
	return hooks.Hook{
		Name: "tool_result_guardrail", Point: hooks.PreModelRequest,
		On: func(s hooks.State) *hooks.Result {
			count := s.Get(hooks.StoreToolResultCount)
			lastWarned := s.Get(hooks.CounterToolResultWarned)
			if count <= toolResultThreshold || count-lastWarned < toolResultInterval {
				return nil
			}
			s.Set(hooks.CounterToolResultWarned, count)
			return &hooks.Result{Hint: &hooks.Hint{
				Type:     "tool_results",
				Severity: "warning",
				Content:  fmt.Sprintf("%d tool results accumulated. Check for unfinished sub-tasks - if any, continue with task. If all done, call task(verify) to validate, then report results.", count),
			}}
		},
	}
}

func ReadBeforeWriteHook() hooks.Hook {
	return hooks.Hook{
		Name: "read_before_write", Point: hooks.PreToolUse,
		On: func(s hooks.State) *hooks.Result {
			name := s.ToolName()
			if name != "edit" && name != "write" {
				return nil
			}
			path := s.GetStr(hooks.StoreEditTargetPath)
			if path == "" || s.Get(hooks.StoreEditTargetExists) != 1 || s.Get(hooks.StoreEditTargetWasRead) == 1 {
				return nil
			}
			if name == "edit" && s.Get(hooks.StoreEditAnchorSufficient) == 1 {
				return nil
			}
			return &hooks.Result{BlockTool: &hooks.BlockTool{
				Tool:   name,
				Reason: "你正在修改 " + path + "，但 ledger 中没有该文件的读取记录。请先 Read 确认当前内容，确认差异后再 edit/write。",
			}}
		},
	}
}

func BashLsGuardrailHook() hooks.Hook {
	return hooks.Hook{
		Name: "bash_ls_guardrail", Point: hooks.PreToolUse,
		On: func(s hooks.State) *hooks.Result {
			if s.ToolName() != "bash" {
				return nil
			}
			cmd, _ := s.ToolArgs()["command"].(string)
			if !isLsCommand(cmd) {
				return nil
			}
			return &hooks.Result{BlockTool: &hooks.BlockTool{
				Tool:   "bash",
				Reason: "不要用 bash 执行 ls 探索目录。请改用 list 工具读取目标目录；如果目标不是目录，先按文件处理。",
			}}
		},
	}
}

func isLsCommand(cmd string) bool {
	fields := strings.Fields(strings.TrimSpace(cmd))
	return len(fields) > 0 && fields[0] == "ls"
}

func ReadOnlySpiralHook() hooks.Hook {
	return hooks.Hook{
		Name: "read_only_spiral", Point: hooks.PostTool,
		On: func(s hooks.State) *hooks.Result {
			if s.Get(hooks.StoreReadOnlyStreak) < 3 {
				return nil
			}
			s.Set(hooks.StoreReadOnlyStreak, 0)
			return &hooks.Result{Hint: &hooks.Hint{
				Type:     "read_only_spiral",
				Severity: "warning",
				Content:  "You've been reading without acting. Summarize your findings now - don't read any more files.",
			}}
		},
	}
}
