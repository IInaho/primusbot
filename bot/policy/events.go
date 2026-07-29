package policy

// Turn describes the information policy needs when a turn starts.
type Turn struct {
	Input     string
	HasTasks  bool
	TasksDone bool
}

// ToolRequest describes a tool before execution. TargetExists is supplied by
// the tool runtime because path resolution belongs to the tool layer.
type ToolRequest struct {
	Name         string
	Args         map[string]any
	TargetExists bool
}

// ToolResult is the single event recorded after a tool call.
type ToolResult struct {
	Name        string
	Args        map[string]any
	Output      string
	Error       string
	Blocked     bool
	BlockReason string
}

type FinalIntent string

const (
	FinalIntentFinal       FinalIntent = "final"
	FinalIntentError       FinalIntent = "error"
	FinalIntentFormatError FinalIntent = "format_error"
	FinalIntentNonFinal    FinalIntent = "non_final"
)

type TurnResult struct {
	Intent  FinalIntent
	Garbled bool
}

// Facts is the immutable view evaluated by hooks.
type Facts struct {
	Turn        TurnFacts
	Tool        ToolFacts
	Activity    ActivityFacts
	Exploration ExplorationFacts
	Model       ModelFacts
	Response    ResponseFacts
}

type TurnFacts struct {
	Input     string
	ReadsLeft int
	HasTasks  bool
	TasksDone bool
}

type ToolFacts struct {
	Name                 string
	Args                 map[string]any
	Error                bool
	TargetPath           string
	TargetExists         bool
	TargetWasRead        bool
	EditAnchorSufficient bool
}

type ActivityFacts struct {
	ToolCalls       int
	ExploreCalls    int
	ResearcherCalls int
	HasEdits        bool
	HasProgress     bool
	ReadOnlyStreak  int
}

type ExplorationFacts struct {
	Score int64
}

type ModelFacts struct {
	ToolResults int
}

type ResponseFacts struct {
	Intent       FinalIntent
	GarbledCount int
}
