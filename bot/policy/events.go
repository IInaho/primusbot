package policy

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
	Tool     ToolFacts
	Response ResponseFacts
}

type ToolFacts struct {
	Name          string
	Args          map[string]any
	Error         bool
	TargetPath    string
	TargetExists  bool
	TargetWasRead bool
}

type ResponseFacts struct {
	Intent       FinalIntent
	GarbledCount int
}
