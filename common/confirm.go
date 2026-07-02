package common

// ConfirmRequest is sent to the TUI when a tool requires user approval.
type ConfirmRequest struct {
	ToolName string
	Args     map[string]any
	Level    DangerLevel
	Response chan ConfirmReply
	// CanEscalatePermission indicates that this tool may require permission
	// escalation after execution. When true, the UI should offer an option
	// to pre-approve permission escalation (merged into the same dialog).
	CanEscalatePermission bool
}

type ConfirmReply struct {
	Allowed  bool
	Remember bool
	// AllowWithPermission indicates that the user wants to both allow the
	// tool execution AND pre-approve any permission escalation that might
	// be needed. This avoids a second confirmation dialog.
	AllowWithPermission bool
}

func AllowOnce() ConfirmReply {
	return ConfirmReply{Allowed: true}
}

func AllowRemembered() ConfirmReply {
	return ConfirmReply{Allowed: true, Remember: true}
}

func AllowWithPermission() ConfirmReply {
	return ConfirmReply{Allowed: true, Remember: true, AllowWithPermission: true}
}

func Deny() ConfirmReply {
	return ConfirmReply{}
}

// NewConfirmRequest creates a ConfirmRequest with an initialized response channel.
func NewConfirmRequest(toolName string, args map[string]any, level DangerLevel) ConfirmRequest {
	return ConfirmRequest{
		ToolName: toolName,
		Args:     args,
		Level:    level,
		Response: make(chan ConfirmReply, 1),
	}
}

// ConfirmFunc asks the user to approve a tool call.
type ConfirmFunc func(req ConfirmRequest) ConfirmReply

// PhaseFunc is called when the agent's phase changes.
type PhaseFunc func(phase string)

// Phase constants — emitted by agent, displayed by TUI status line.
const (
	PhaseReady     = "Ready"
	PhaseWaiting   = "Waiting"
	PhaseThinking  = "Thinking"
	PhaseReasoning = "Reasoning"
	PhaseRunning   = "Running"
)
