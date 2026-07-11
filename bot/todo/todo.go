package todo

// Item represents a single task in the todo list.
type Item struct {
	Content string `json:"content"`
	Status  string `json:"status"` // "pending", "in_progress", "completed"
}

// Func is called whenever the todo list is updated.
type Func func(items []Item)

func CountCompleted(items []Item) int {
	n := 0
	for _, it := range items {
		if it.Status == "completed" {
			n++
		}
	}
	return n
}

func StatusIcon(status string) string {
	switch status {
	case "in_progress":
		return "▸"
	case "completed":
		return "✓"
	default:
		return "·"
	}
}
