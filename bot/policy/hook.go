package policy

// HookPoint identifies one concrete place in the agent lifecycle.
type HookPoint string

const (
	PreTurn         HookPoint = "pre_turn"
	PreModelRequest HookPoint = "pre_model_request"
	PreToolUse      HookPoint = "pre_tool_use"
	PostToolUse     HookPoint = "post_tool_use"
	PostTool        HookPoint = "post_tool"
	PostTurn        HookPoint = "post_turn"
	UserSubmit      HookPoint = "user_submit"
	Stop            HookPoint = "stop"
)

type Hint struct {
	Type     string
	Severity string
	Content  string
}

type StopReason string

const (
	StopFormatError StopReason = "format_error"
	StopInterrupted StopReason = "interrupted"
	StopCompleted   StopReason = "completed"
)

func (s StopReason) String() string { return string(s) }

type Result struct {
	Hint        *Hint
	Stop        *StopReason
	BlockTool   *BlockTool
	RequireTool *RequireTool
	BlockFinal  *BlockFinal
}

type BlockTool struct {
	Tool   string
	Reason string
}

type RequireTool struct {
	Tool   string
	Reason string
}

type BlockFinal struct {
	Reason string
}

// Hook is the extension contract exposed by policy. Facts are read-only;
// Int/String store state private to this hook for deduplication and latches.
type Hook struct {
	Name            string
	Point           HookPoint
	On              func(State) *Result
	DescribeTrigger func(State) string
}

type State interface {
	Facts() Facts
	Int(name string) int64
	SetInt(name string, value int64)
	String(name string) string
	SetString(name, value string)
}
