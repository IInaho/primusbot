package view

type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

type TodoFunc func(items []TodoItem)

func CountCompleted(items []TodoItem) int {
	n := 0
	for _, it := range items {
		if it.Status == "completed" {
			n++
		}
	}
	return n
}

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
