package runtime

import (
	"context"

	"nekocode/protocol"
)

// Runner is the only capability required by Manager.
//
// A runner receives a host scoped to exactly one run. It must not retain the
// host after Run returns. A Runner or command callback must not call
// Manager.Close synchronously; shutdown is owned by the application lifecycle.
type Runner interface {
	Run(ctx context.Context, input string, host RunHost) (string, error)
}

// RunnerFunc adapts a function into a Runner for small applications.
type RunnerFunc func(context.Context, string, RunHost) (string, error)

func (f RunnerFunc) Run(ctx context.Context, input string, host RunHost) (string, error) {
	return f(ctx, input, host)
}

// RunHost is the bot-facing boundary for streaming output and requesting
// interaction during one run. Confirm and Ask are serialized so interaction
// surfaces only need to present one pending request at a time.
type RunHost interface {
	Text(delta string)
	Reason(delta string)
	Tool(event ToolEvent)
	SubAgent(event SubAgentEvent)
	Phase(phase string)
	Todos(items []TodoItem)
	Confirm(request ConfirmRequest) ConfirmReply
	Ask(request QuestionRequest) QuestionReply
}

type ToolEventKind string

const (
	ToolEventStarted   ToolEventKind = "started"
	ToolEventBlocked   ToolEventKind = "blocked"
	ToolEventPreview   ToolEventKind = "preview"
	ToolEventCompleted ToolEventKind = "completed"
)

// ToolEvent describes one tool transition. SubAgentID is set only when the
// tool belongs to a child agent.
type ToolEvent struct {
	Kind          ToolEventKind
	CallID        string
	Name          string
	Args          string
	Output        string
	Preview       string
	IsError       bool
	SubAgentID    string
	SubAgentColor int
}

type SubAgentEventKind string

const (
	SubAgentEventStarted SubAgentEventKind = "started"
	SubAgentEventEnded   SubAgentEventKind = "ended"
)

// SubAgentEvent describes the lifecycle of one child agent.
type SubAgentEvent struct {
	Kind  SubAgentEventKind
	ID    string
	Type  string
	Color int
}

// CommandAction describes what runtime should do after a bot command.
type CommandAction = protocol.CommandAction

const (
	CommandIgnored  = protocol.CommandIgnored
	CommandHandled  = protocol.CommandHandled
	CommandContinue = protocol.CommandContinue
)

type CommandResult = protocol.CommandResult
type CommandMenu = protocol.CommandMenu
type CommandMenuItem = protocol.CommandMenuItem
