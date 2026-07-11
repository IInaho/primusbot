package view

import "nekocode/bot/todo"

type TodoItem = todo.Item
type TodoFunc = todo.Func

func CountCompleted(items []TodoItem) int {
	return todo.CountCompleted(items)
}

func TodoStatusIcon(status string) string {
	return todo.StatusIcon(status)
}
