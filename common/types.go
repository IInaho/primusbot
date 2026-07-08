// types.go — shared protocol types used by both bot and UI layers.
//
// UI data types (DisplayMessage, BotStats, ContextSnapshot, CmdResult, ...)
// live in github.com/nekocode/common/ui. This file keeps type aliases so
// existing consumers continue to compile; new code should import common/ui
// directly.
package common

import "nekocode/common/ui"

// CmdResult tells the TUI what to do after a command is executed.
type CmdResult = ui.CmdResult

const (
	CmdNone           = ui.CmdNone
	CmdHandled        = ui.CmdHandled
	CmdConfirming     = ui.CmdConfirming
	CmdSessionResumed = ui.CmdSessionResumed
)

// BotStats carries runtime statistics from the bot to the UI.
type BotStats = ui.BotStats

// ContextSegment describes one visible part of the active context window.
type ContextSegment = ui.ContextSegment

// ContextSnapshot is the structured context status consumed by UI surfaces.
type ContextSnapshot = ui.ContextSnapshot

type RunCallbacks struct {
	Text   func(delta string)
	Reason func(delta string)
	Step   func(action, toolName, toolArgs, output string)
}

// TodoItem represents a single task in the todo list.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // "pending", "in_progress", "completed"
}

// TodoFunc is called whenever the todo list is updated.
type TodoFunc func(items []TodoItem)

// CountCompleted returns the number of completed items.
func CountCompleted(items []TodoItem) int {
	n := 0
	for _, it := range items {
		if it.Status == "completed" {
			n++
		}
	}
	return n
}

// TodoStatusIcon returns the display icon for a todo status.
func TodoStatusIcon(status string) string {
	switch status {
	case "in_progress":
		return "▸"
	case "completed":
		return "✓"
	default:
		return "·"
	}
}

// SubSlot tracks an active sub-agent for rendering and slot management.
type SubSlot = ui.SubSlot

// DisplayBlock carries a persistent tool result for UI rendering.
type DisplayBlock = ui.DisplayBlock

// ImageRef carries a generated image reference for UI rendering.
type ImageRef = ui.ImageRef

// DisplayMessage is a lightweight message representation for the UI layer.
type DisplayMessage = ui.DisplayMessage
