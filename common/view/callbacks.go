package view

type StepAction string

const (
	StepActionChat           StepAction = "chat"
	StepActionThink          StepAction = "think"
	StepActionToolStart      StepAction = "tool_start"
	StepActionToolBlocked    StepAction = "tool_blocked"
	StepActionToolPreview    StepAction = "tool_preview"
	StepActionExecuteTool    StepAction = "execute_tool"
	StepActionSubToolStart   StepAction = "sub_tool_start"
	StepActionSubExecuteTool StepAction = "sub_execute_tool"
	StepActionSubAgentStart  StepAction = "sub_agent_start"
	StepActionSubAgentEnd    StepAction = "sub_agent_end"
)

// StepEvent carries one step emitted during a run.
type StepEvent struct {
	Action   StepAction
	CallID   string // tool call ID; empty for non-tool steps
	ToolName string
	ToolArgs string
	Output   string
	IsError  bool // true when a tool result is an error
}

type RunCallbacks struct {
	Text   func(delta string)
	Reason func(delta string)
	Step   func(ev StepEvent)
}

type ControlCallbacks struct {
	Confirm   ConfirmFunc
	Phase     PhaseFunc
	Todo      TodoFunc
	Notify    func(string)
	ConfirmCh chan ConfirmRequest
	Question  QuestionFunc
}
