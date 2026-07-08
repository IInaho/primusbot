// Package ui holds data-transfer types that bot produces and UI layers
// (tui/gui) consume. Keeping them separate from the protocol types in
// common makes the bot↔UI data boundary explicit.
package ui

// DisplayBlock carries a persistent tool result for UI rendering.
type DisplayBlock struct {
	ToolName string
	Args     string
	Content  string
	IsError  bool
}

// ImageRef carries a generated image reference for UI rendering.
type ImageRef struct {
	Path   string
	URL    string
	Width  int
	Height int
}

// DisplayMessage is a lightweight message representation for the UI layer
// to reconstruct chat history from a restored session.
type DisplayMessage struct {
	Role    string
	Content string
	Blocks  []DisplayBlock
	Images  []ImageRef
}

// SubSlot tracks an active sub-agent for rendering and slot management.
type SubSlot struct {
	ID       string
	SubType  string
	ColorIdx int
}
