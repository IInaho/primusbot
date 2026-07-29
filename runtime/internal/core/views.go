package core

import "time"

type RunView struct {
	ID         RunID          `json:"id"`
	Status     RunStatus      `json:"status"`
	Source     SourceRef      `json:"source"`
	Sender     SenderRef      `json:"sender"`
	Input      string         `json:"input,omitempty"`
	Phase      string         `json:"phase,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Tools      []ToolView     `json:"tools,omitempty"`
	Approvals  []ApprovalView `json:"approvals,omitempty"`
	Questions  []QuestionView `json:"questions,omitempty"`
	Output     string         `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	EventCount int            `json:"event_count"`
}

type ToolStatus string

const (
	ToolRunning ToolStatus = "running"
	ToolDone    ToolStatus = "done"
	ToolBlocked ToolStatus = "blocked"
)

type ToolView struct {
	Name       string     `json:"name"`
	Args       string     `json:"args,omitempty"`
	Preview    string     `json:"preview,omitempty"`
	Output     string     `json:"output,omitempty"`
	Status     ToolStatus `json:"status"`
	IsError    bool       `json:"is_error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type ArtifactView struct {
	RunID   RunID          `json:"run_id"`
	Diffs   []ArtifactItem `json:"diffs,omitempty"`
	Patches []ArtifactItem `json:"patches,omitempty"`
	Reviews []ArtifactItem `json:"reviews,omitempty"`
	Results []ArtifactItem `json:"results,omitempty"`
	Errors  []ArtifactItem `json:"errors,omitempty"`
	Events  int            `json:"events"`
}

type ArtifactItem struct {
	Kind      string    `json:"kind"`
	ToolName  string    `json:"tool_name,omitempty"`
	Title     string    `json:"title,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ConnectorStatusPayload struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ConnectView struct {
	Connectors []ConnectorView `json:"connectors"`
}

type ConnectorView struct {
	Name        string                `json:"name"`
	Registered  bool                  `json:"registered"`
	Initialized bool                  `json:"initialized"`
	Configured  bool                  `json:"configured"`
	Running     bool                  `json:"running"`
	Status      string                `json:"status"`
	Message     string                `json:"message,omitempty"`
	Devices     []ConnectorDeviceView `json:"devices,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty"`
}

type ConnectorDeviceView struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Display  string `json:"display,omitempty"`
	LastSeen int64  `json:"last_seen,omitempty"`
	PairedAt int64  `json:"paired_at,omitempty"`
}
