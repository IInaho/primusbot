// Package protocol defines the transport- and UI-neutral contract shared by
// the bot foundation and runtime adapters.
package protocol

type PhaseFunc func(string)

const (
	PhaseReady     = "Ready"
	PhaseWaiting   = "Waiting"
	PhaseThinking  = "Thinking"
	PhaseReasoning = "Reasoning"
	PhaseRunning   = "Running"
)

type StepAction string

const (
	StepActionChat          StepAction = "chat"
	StepActionThink         StepAction = "think"
	StepActionToolStart     StepAction = "tool_start"
	StepActionToolBlocked   StepAction = "tool_blocked"
	StepActionToolPreview   StepAction = "tool_preview"
	StepActionExecuteTool   StepAction = "execute_tool"
	StepActionSubAgentStart StepAction = "sub_agent_start"
	StepActionSubAgentEnd   StepAction = "sub_agent_end"
)

type StepEvent struct {
	Action        StepAction
	CallID        string
	ToolName      string
	ToolArgs      string
	Output        string
	IsError       bool
	SubAgentID    string
	SubAgentType  string
	SubAgentColor int
}

type CommandAction string

const (
	CommandIgnored  CommandAction = "ignored"
	CommandHandled  CommandAction = "handled"
	CommandContinue CommandAction = "continue"
)

type CommandResult struct {
	Action     CommandAction
	Output     string
	AgentInput string
}

// LocalCommandResult classifies the outcome of ExecuteLocalCommand: commands
// that neither read nor mutate conversation context run immediately, even
// while a task is in progress; everything else goes through the run path.
type LocalCommandResult int

const (
	// LocalCommandNotCommand: input is not a registered command.
	LocalCommandNotCommand LocalCommandResult = iota
	// LocalCommandExecuted: a during-task-safe command ran immediately.
	LocalCommandExecuted
	// LocalCommandRequiresIdle: input is a command but needs the run path,
	// and therefore an idle runtime.
	LocalCommandRequiresIdle
)

// CommandMenu is a transport-neutral, dynamically generated command picker.
// Each item contains the complete input to place in the command line.
type CommandMenu struct {
	Title string            `json:"title"`
	Empty string            `json:"empty,omitempty"`
	Items []CommandMenuItem `json:"items"`
}

type CommandMenuItem struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Submit      bool   `json:"submit,omitempty"`
}

type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

type TodoFunc func([]TodoItem)

func CountCompleted(items []TodoItem) int {
	n := 0
	for _, item := range items {
		if item.Status == "completed" {
			n++
		}
	}
	return n
}

type ConfirmKind string

const (
	ConfirmKindPermission ConfirmKind = "permission"
	ConfirmKindInstall    ConfirmKind = "install"
)

type ConfirmRequest struct {
	ToolName              string
	Args                  map[string]any
	Kind                  ConfirmKind
	CanEscalatePermission bool
}

type ConfirmReply struct {
	Allowed             bool
	Remember            bool
	AllowWithPermission bool
}

func AllowOnce() ConfirmReply       { return ConfirmReply{Allowed: true} }
func AllowRemembered() ConfirmReply { return ConfirmReply{Allowed: true, Remember: true} }
func Deny() ConfirmReply            { return ConfirmReply{} }

func NewConfirmRequest(toolName string, args map[string]any, kind ConfirmKind) ConfirmRequest {
	return ConfirmRequest{
		ToolName: toolName,
		Args:     args,
		Kind:     kind,
	}
}

type ConfirmFunc func(ConfirmRequest) ConfirmReply

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type QuestionItem struct {
	Header   string           `json:"header,omitempty"`
	Question string           `json:"question"`
	Options  []QuestionOption `json:"options"`
	Multiple bool             `json:"multiple,omitempty"`
	Custom   bool             `json:"custom,omitempty"`
}

type QuestionReply struct {
	Answers  [][]string `json:"answers,omitempty"`
	Rejected bool       `json:"rejected,omitempty"`
}

type QuestionRequest struct {
	Questions []QuestionItem
}

func NewQuestionRequest(questions []QuestionItem) QuestionRequest {
	return QuestionRequest{
		Questions: questions,
	}
}

type QuestionFunc func(QuestionRequest) QuestionReply

// Metrics is the bot's operational measurement snapshot.
type Metrics struct {
	PromptTokens      int    `json:"promptTokens"`
	CompletionTokens  int    `json:"completionTokens"`
	TurnPrompt        int    `json:"turnPrompt"`
	TurnCompletion    int    `json:"turnCompletion"`
	TurnTotal         int    `json:"turnTotal"`
	TurnInput         int    `json:"turnInput"`
	TurnCached        int    `json:"turnCached"`
	TurnNew           int    `json:"turnNew"`
	TurnOutput        int    `json:"turnOutput"`
	TurnReasoning     int    `json:"turnReasoning,omitempty"`
	TurnCacheReported bool   `json:"turnCacheReported"`
	ContextTokens     int    `json:"contextTokens"`
	CompactCount      int    `json:"compactCount"`
	Duration          string `json:"duration"`
}

// HasTurnUsage reports whether the snapshot contains an actual LLM request.
// Command-only runs intentionally carry no turn usage.
func (m Metrics) HasTurnUsage() bool { return m.TurnInput > 0 || m.TurnOutput > 0 }

// WorkspaceChanges is a privacy-safe summary of the current Git worktree.
type WorkspaceChanges struct {
	Added     int  `json:"added"`
	Deleted   int  `json:"deleted"`
	Untracked int  `json:"untracked"`
	Available bool `json:"available"`
}
