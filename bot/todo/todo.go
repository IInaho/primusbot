// Package todo re-exports the shared todo item model from common/view.
package todo

import commonview "nekocode/common/view"

// Item represents a single task in the todo list.
type Item = commonview.TodoItem

// Func is called whenever the todo list is updated.
type Func = commonview.TodoFunc

var (
	CountCompleted = commonview.CountCompleted
	StatusIcon     = commonview.TodoStatusIcon
)
