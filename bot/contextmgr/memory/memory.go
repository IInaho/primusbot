// Package memory loads persistent project context for prompt injection.
//
// The Markdown file uses five fixed sections:
//
//	## Tech Stack
//	## Active Goals
//	## Completed Tasks
//	## Key Architecture Map
//	## User Preferences
package memory

import (
	"os"
	"path/filepath"
	"strings"

	"nekocode/util/fs"
)

// File is the parsed, read-only project memory.
type File struct {
	TechStack      string
	ActiveGoals    string
	CompletedTasks string
	ArchMap        string
	Preferences    string
}

type section struct {
	key    string
	header string
}

var sections = []section{
	{"TechStack", "## Tech Stack"},
	{"ActiveGoals", "## Active Goals"},
	{"CompletedTasks", "## Completed Tasks"},
	{"ArchMap", "## Key Architecture Map"},
	{"Preferences", "## User Preferences"},
}

func DefaultPath() string {
	return filepath.Join(fs.NekocodeHome(), "memory.md")
}

// Load reads a project memory file. A missing file produces an empty model.
func Load(path string) (*File, error) {
	f := &File{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return nil, err
	}
	f.parse(string(data))
	return f, nil
}

// Build formats non-empty sections for the immutable prompt prefix.
func (f *File) Build() string {
	var b strings.Builder
	b.WriteString("[Project Memory]\n")
	hasContent := false
	for _, sec := range sections {
		content := strings.TrimSpace(f.getField(sec.key))
		if content == "" {
			continue
		}
		b.WriteString(sec.header)
		b.WriteByte('\n')
		b.WriteString(content)
		b.WriteString("\n\n")
		hasContent = true
	}
	if !hasContent {
		return ""
	}
	return b.String()
}

func (f *File) parse(data string) {
	current := ""
	for line := range strings.SplitSeq(data, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, sec := range sections {
			if trimmed == sec.header {
				current = sec.key
				break
			}
		}
		if current == "" || strings.HasPrefix(trimmed, "##") {
			continue
		}
		existing := f.getField(current)
		if existing == "" {
			f.setField(current, trimmed)
		} else {
			f.setField(current, existing+"\n"+trimmed)
		}
	}
}

func (f *File) getField(key string) string {
	switch key {
	case "TechStack":
		return f.TechStack
	case "ActiveGoals":
		return f.ActiveGoals
	case "CompletedTasks":
		return f.CompletedTasks
	case "ArchMap":
		return f.ArchMap
	case "Preferences":
		return f.Preferences
	default:
		return ""
	}
}

func (f *File) setField(key, value string) {
	switch key {
	case "TechStack":
		f.TechStack = value
	case "ActiveGoals":
		f.ActiveGoals = value
	case "CompletedTasks":
		f.CompletedTasks = value
	case "ArchMap":
		f.ArchMap = value
	case "Preferences":
		f.Preferences = value
	}
}
