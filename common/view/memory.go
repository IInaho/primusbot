package view

import "strings"

type MemoryScope string

const (
	MemoryScopeProject MemoryScope = "project"
)

type MemorySection struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Empty   bool   `json:"empty"`
}

type MemoryView struct {
	Scope    MemoryScope     `json:"scope"`
	Path     string          `json:"path,omitempty"`
	Content  string          `json:"content"`
	Sections []MemorySection `json:"sections,omitempty"`
	Empty    bool            `json:"empty"`
}

func NewMemoryView(scope MemoryScope, path, content string) MemoryView {
	content = strings.TrimSpace(content)
	if scope == "" {
		scope = MemoryScopeProject
	}
	return MemoryView{
		Scope:    scope,
		Path:     path,
		Content:  content,
		Sections: ParseMemorySections(content),
		Empty:    content == "",
	}
}

func ParseMemorySections(content string) []MemorySection {
	sections := canonicalMemorySections()
	byTitle := make(map[string]int, len(sections))
	for i, sec := range sections {
		byTitle[sec.Title] = i
	}

	current := -1
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "[Project Memory]" {
			continue
		}
		if idx, ok := byTitle[trimmed]; ok {
			current = idx
			continue
		}
		if current < 0 {
			continue
		}
		if sections[current].Content == "" {
			sections[current].Content = line
		} else {
			sections[current].Content += "\n" + line
		}
	}

	for i := range sections {
		sections[i].Content = strings.TrimSpace(sections[i].Content)
		sections[i].Empty = sections[i].Content == ""
	}
	return sections
}

func canonicalMemorySections() []MemorySection {
	return []MemorySection{
		{Key: "tech_stack", Title: "## Tech Stack", Empty: true},
		{Key: "active_goals", Title: "## Active Goals", Empty: true},
		{Key: "completed_tasks", Title: "## Completed Tasks", Empty: true},
		{Key: "architecture_map", Title: "## Key Architecture Map", Empty: true},
		{Key: "preferences", Title: "## User Preferences", Empty: true},
	}
}
