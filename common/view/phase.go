package view

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
