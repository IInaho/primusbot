package view

// ConfirmRequest is sent to UI clients when a tool requires user approval.
type ConfirmRequest struct {
	ToolName string
	Args     map[string]any
	Kind     ConfirmKind
	Response chan ConfirmReply
	// CanEscalatePermission indicates that this tool may require permission
	// escalation after execution.
	CanEscalatePermission bool
}

type ConfirmKind string

const (
	ConfirmKindPermission ConfirmKind = "permission"
	ConfirmKindInstall    ConfirmKind = "install"
)

type ConfirmReply struct {
	Allowed  bool
	Remember bool
	// AllowWithPermission pre-approves a permission escalation for the same call.
	AllowWithPermission bool
}

func AllowOnce() ConfirmReply {
	return ConfirmReply{Allowed: true}
}

func AllowRemembered() ConfirmReply {
	return ConfirmReply{Allowed: true, Remember: true}
}

func Deny() ConfirmReply {
	return ConfirmReply{}
}

func NewConfirmRequest(toolName string, args map[string]any, kind ConfirmKind) ConfirmRequest {
	return ConfirmRequest{
		ToolName: toolName,
		Args:     args,
		Kind:     kind,
		Response: make(chan ConfirmReply, 1),
	}
}

// ConfirmFunc asks the user to approve a tool call.
type ConfirmFunc func(req ConfirmRequest) ConfirmReply

// PhaseFunc is called when the agent's phase changes.
type PhaseFunc func(phase string)

// Phase constants emitted by agent runtime and displayed by UI surfaces.
const (
	PhaseReady     = "Ready"
	PhaseWaiting   = "Waiting"
	PhaseThinking  = "Thinking"
	PhaseReasoning = "Reasoning"
	PhaseRunning   = "Running"
)
