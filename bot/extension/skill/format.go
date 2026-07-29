package skill

import (
	"fmt"
	"strings"
)

// Skill list text budget, in characters: contextWindow/100 clamped to this range.
const (
	minListChars = 500
	maxListChars = 3000
)

const skillListHeader = "## Available Skills (complete — no need to search for more)\n\n" +
	"This is the authoritative list. Do NOT glob/grep/list to find skills — trust this list. Loaded skills are excluded:\n\n"

// buildSkillListText generates the available-skills text injected into context.
// The first eligible entry is always written, even if it exceeds the budget;
// once the budget is exhausted the remaining skills are only counted.
func buildSkillListText(skills []*Skill, loaded map[string]bool, contextWindow int) string {
	total := 0
	for _, sk := range skills {
		if !loaded[sk.Name] {
			total++
		}
	}
	if total == 0 {
		return ""
	}

	maxChars := max(min(contextWindow/100, maxListChars), minListChars)
	remaining := maxChars - len([]rune(skillListHeader))

	var sb strings.Builder
	sb.WriteString(skillListHeader)
	listed := 0
	for _, sk := range skills {
		if loaded[sk.Name] {
			continue
		}
		entry := fmt.Sprintf("- **%s**: %s\n", sk.Name, sk.Description)
		if listed > 0 && remaining < len([]rune(entry)) {
			break
		}
		sb.WriteString(entry)
		remaining -= len([]rune(entry))
		listed++
	}
	if listed < total {
		fmt.Fprintf(&sb, "\n(%d more skills available but omitted due to token budget)\n", total-listed)
	}
	return sb.String()
}

// FormatForContext formats a skill's content for injection into conversation context.
func FormatForContext(sk *Skill) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<skill_content name=\"%s\">\n# Skill: %s\n\n", sk.Name, sk.Name)
	fmt.Fprintf(&sb, "**This skill is already loaded. Do NOT call the skill tool for %q.**\n\n", sk.Name)

	if sk.Dir != "" {
		fmt.Fprintf(&sb, "**Skill files: `%s`** — Read input files using absolute paths. Do NOT glob or search.\n", sk.Dir)
		sb.WriteString("**Output files go to the current working directory**, NOT the skill directory.\n\n")
	} else {
		sb.WriteString("(This is a built-in skill with no file-system directory.)\n\n")
	}
	sb.WriteString(sk.Content)

	if len(sk.Files) > 0 {
		sb.WriteString("\n\n## Skill files (absolute paths):\n")
		for _, f := range sk.Files {
			fmt.Fprintf(&sb, "- `%s`\n", f)
		}
	}
	sb.WriteString("</skill_content>")
	return sb.String()
}
