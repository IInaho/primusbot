// Package memory manages the persistent project memory file.
// Memory is injected into Layer 0 (immutable prefix) and only changes
// on explicit /remember commands or Layer 5 Auto-Compaction.
//
// Five sections, stored as a markdown file:
//
//	## Tech Stack           — languages, frameworks, infrastructure
//	## Active Goals         — current tasks in progress
//	## Completed Tasks      — milestones achieved
//	## Key Architecture Map — component → responsibility mappings
//	## User Preferences     — explicit user rules and preferences
//
// TODO: consider migrating from fixed-field struct to a general key-value
// store for better extensibility (e.g. map[string]string).
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nekocode/util/fs"
)

// File is the in-memory representation of the memory file.
type File struct {
	mu   sync.RWMutex
	path string

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

// sections is the canonical parse/build/save order.
var sections = []section{
	{"TechStack", "## Tech Stack"},
	{"ActiveGoals", "## Active Goals"},
	{"CompletedTasks", "## Completed Tasks"},
	{"ArchMap", "## Key Architecture Map"},
	{"Preferences", "## User Preferences"},
}

// DefaultPath returns the default memory file path.
func DefaultPath() string {
	return filepath.Join(fs.NekocodeHome(), "memory.md")
}

// Load reads the memory file from disk. Returns an empty File if none exists.
func Load(path string) (*File, error) {
	f := &File{path: path}
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

// Build returns the formatted memory block for Layer 0 injection.
func (f *File) Build() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var b strings.Builder
	b.WriteString("[Project Memory]\n")
	hasContent := false

	for _, sec := range sections {
		content := strings.TrimSpace(f.getField(sec.key))
		if content == "" {
			continue
		}
		b.WriteString(sec.header)
		b.WriteString("\n")
		b.WriteString(content)
		b.WriteString("\n\n")
		hasContent = true
	}
	if !hasContent {
		return ""
	}
	return b.String()
}

// Save writes the memory file to disk.
func (f *File) Save() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var b strings.Builder
	for _, sec := range sections {
		content := f.getField(sec.key)
		b.WriteString(sec.header)
		b.WriteString("\n")
		if strings.TrimSpace(content) == "" {
			b.WriteString("\n")
		} else {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}
	return fs.WriteFileWithDir(f.path, []byte(b.String()), 0o644)
}

// Append adds a line to a section. Used by /remember.
func (f *File) Append(section, line string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := f.resolveSection(section)
	if key == "" {
		return fmt.Errorf("unknown section: %s (valid: tech-stack, goals, completed, architecture, preferences)", section)
	}

	current := f.getField(key)
	if strings.TrimSpace(current) == "" {
		f.setField(key, "- "+line)
	} else {
		f.setField(key, current+"\n- "+line)
	}
	return nil
}

// MergeFromCompaction updates memory from Auto-Compaction output.
// newFacts are lines extracted from the summarizer's <key-facts> block;
// goal is the updated current goal; archMap entries are derived from facts.
func (f *File) MergeFromCompaction(newFacts []string, goal string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if strings.TrimSpace(goal) != "" {
		f.ActiveGoals = "- " + goal
	}

	for _, fact := range newFacts {
		fact = strings.TrimSpace(fact)
		if fact == "" || containsLine(f.ArchMap, fact) {
			continue
		}
		if strings.TrimSpace(f.ArchMap) == "" {
			f.ArchMap = "- " + fact
		} else {
			f.ArchMap += "\n- " + fact
		}
	}
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
		if current != "" && !strings.HasPrefix(trimmed, "##") {
			existing := f.getField(current)
			if existing == "" {
				f.setField(current, trimmed)
			} else {
				f.setField(current, existing+"\n"+trimmed)
			}
		}
	}
}

func (f *File) resolveSection(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "tech-stack", "techstack", "tech":
		return "TechStack"
	case "goals", "active-goals", "goal":
		return "ActiveGoals"
	case "completed", "completed-tasks", "done":
		return "CompletedTasks"
	case "architecture", "arch", "architecture-map":
		return "ArchMap"
	case "preferences", "prefs", "user-preferences":
		return "Preferences"
	default:
		return ""
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

func containsLine(haystack, needle string) bool {
	needle = cleanListLine(needle)
	for line := range strings.SplitSeq(haystack, "\n") {
		if cleanListLine(line) == needle {
			return true
		}
	}
	return false
}

func cleanListLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	line = strings.TrimPrefix(line, "• ")
	return line
}
