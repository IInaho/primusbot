package builtin

import (
	"fmt"

	"nekocode/bot/policy"
)

// dashIfEmpty renders an empty string as "-" in audit trigger output, matching
// the registry's formatting convention for missing values.
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func All() []policy.Hook {
	return []policy.Hook{
		ReadBeforeWriteHook(),
		GarbledCircuitBreaker(),
	}
}

// Register installs all built-in hooks into p.
func Register(p *policy.Policy) {
	for _, h := range All() {
		p.Register(h)
	}
}

// ReadBeforeWriteHook requires a successful dedicated read before modifying an
// existing file. It deliberately does not infer reads from shell command text
// or accept an edit anchor as equivalent evidence.
func ReadBeforeWriteHook() policy.Hook {
	return policy.Hook{
		Name: "read_before_write", Point: policy.PreToolUse,
		On: func(s policy.State) *policy.Result {
			tool := s.Facts().Tool
			name := tool.Name
			if name != "edit" && name != "write" {
				return nil
			}
			path := tool.TargetPath
			if path == "" || !tool.TargetExists || tool.TargetWasRead {
				return nil
			}
			return &policy.Result{BlockTool: &policy.BlockTool{
				Tool:   name,
				Reason: "你正在修改 " + path + "，但 ledger 中没有该文件的读取记录。请先 Read 确认当前内容，确认差异后再 edit/write。",
			}}
		},
		DescribeTrigger: func(s policy.State) string {
			tool := s.Facts().Tool
			return fmt.Sprintf("target=%s exists=%t was_read=%t",
				dashIfEmpty(tool.TargetPath), tool.TargetExists, tool.TargetWasRead)
		},
	}
}

func GarbledCircuitBreaker() policy.Hook {
	return policy.Hook{
		Name: "garbled_circuit_breaker", Point: policy.Stop,
		On: func(s policy.State) *policy.Result {
			if s.Facts().Response.GarbledCount >= 5 {
				stop := policy.StopFormatError
				return &policy.Result{Stop: &stop}
			}
			return nil
		},
		DescribeTrigger: func(s policy.State) string {
			return fmt.Sprintf("garbled_count=%d", s.Facts().Response.GarbledCount)
		},
	}
}
