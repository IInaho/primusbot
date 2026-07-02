// confirm_bar.go — 确认弹窗栏（上下键选择 + Enter 确认）。
package components

import (
	"fmt"
	"strings"

	"nekocode/tui/styles"

	"charm.land/lipgloss/v2"

	"nekocode/common"
)

type ConfirmBar struct {
	req      *common.ConfirmRequest
	sty      *styles.Styles
	selected int
}

func NewConfirmBar(sty *styles.Styles) *ConfirmBar {
	return &ConfirmBar{sty: sty, selected: 0}
}

func (c *ConfirmBar) SetRequest(req *common.ConfirmRequest) {
	c.req = req
	c.selected = 0
}

func (c *ConfirmBar) Clear() { c.req = nil }

func (c *ConfirmBar) Selected() int {
	if c.req == nil {
		return 0
	}
	return c.selected
}

func (c *ConfirmBar) Move(delta int) {
	if c.req == nil {
		return
	}
	n := len(c.options())
	if n == 0 {
		return
	}
	c.selected = (c.selected + delta + n) % n
}

func (c *ConfirmBar) Submit() {
	if c.req == nil {
		return
	}
	opts := c.options()
	if c.selected >= len(opts) {
		c.selected = 0
	}
	opts[c.selected].Action(c)
}

func (c *ConfirmBar) Respond(ok bool, remember bool) {
	c.req.Response <- common.ConfirmReply{Allowed: ok, Remember: ok && remember}
	c.req = nil
}

func (c *ConfirmBar) RespondWithPermission(ok bool, remember bool) {
	c.req.Response <- common.ConfirmReply{Allowed: ok, Remember: ok && remember, AllowWithPermission: ok && remember}
	c.req = nil
}

// CanRemember reports whether the user can persist the decision. The legacy
// capability model only persists "project"-scope grants; the new rule engine
// persists an allow Rule for any ask except "once"-scope (CapShellUnknown /
// CapProcessHost), which is intentionally non-persistent.
func (c *ConfirmBar) CanRemember() bool {
	if c.req == nil {
		return false
	}
	scope, _ := c.req.Args["permission_scope"].(string)
	return scope != "once"
}

func (c *ConfirmBar) options() []confirmOption {
	if c.req == nil {
		return nil
	}
	opts := []confirmOption{
		{Label: "仅本次允许", Action: func(c *ConfirmBar) { c.Respond(true, false) }},
	}
	if c.CanRemember() {
		// Privileged tools (bash) may also need capability escalation later,
		// so flag AllowWithPermission; other tools just persist a Rule.
		if c.req.CanEscalatePermission {
			opts = append(opts, confirmOption{Label: "始终允许", Action: func(c *ConfirmBar) { c.RespondWithPermission(true, true) }})
		} else {
			opts = append(opts, confirmOption{Label: "始终允许", Action: func(c *ConfirmBar) { c.Respond(true, true) }})
		}
	}
	opts = append(opts, confirmOption{Label: "拒绝", Action: func(c *ConfirmBar) { c.Respond(false, false) }})
	return opts
}

type confirmOption struct {
	Label  string
	Action func(*ConfirmBar)
}

func confirmMaxLines(termHeight int) int {
	n := termHeight / 3
	if n < 6 {
		n = 6
	}
	return n
}

func (c *ConfirmBar) Height(width, termHeight int) int {
	if c.req == nil {
		return 0
	}
	contentW := max(40, width-6)
	maxLines := confirmMaxLines(termHeight)
	n := len(c.descLines(contentW))
	if n > maxLines {
		n = maxLines + 1
	}
	opts := len(c.options())
	// title(1) + desc(n) + sep(1) + levelTag(1) + opts + navSep(1) + navHint(1) + bottom(1)
	return n + opts + 5
}

func (c *ConfirmBar) View(width, termHeight int) string {
	if c.req == nil {
		return ""
	}
	barW := max(40, width-4)
	contentW := max(40, width-6)
	maxLines := confirmMaxLines(termHeight)

	title := c.sty.Primary.Bold(true).Render("  Confirm")
	prefix := "┌─  Confirm "
	rightLen := max(0, barW-lipgloss.Width(prefix)-1)
	rightDash := c.sty.Border.Render(strings.Repeat(styles.Horizontal, rightLen) + "┐")
	titleBar := c.sty.Border.Render("┌─") + title + " " + rightDash

	desc := c.formatDesc()
	rawLines := wrapText("  "+desc, contentW)
	var descLines []string
	for i, line := range rawLines {
		if i == 0 {
			descLines = append(descLines, line)
		} else {
			descLines = append(descLines, "  "+line)
		}
	}
	truncated := len(descLines) > maxLines
	if truncated {
		descLines = descLines[:maxLines]
	}

	levelTag := c.sty.Yellow.Render("[" + c.req.Level.String() + "]")
	if c.req.Level == common.LevelForbidden {
		levelTag = c.sty.Red.Render("[" + c.req.Level.String() + "]")
	}

	opts := c.options()

	sep := c.sty.Border.Render("├" + strings.Repeat(styles.Horizontal, barW-2) + "┤")
	bottomBorder := c.sty.Border.Render("└" + strings.Repeat(styles.Horizontal, barW-2) + "┘")
	// Light separator between options and the nav hint so the hint reads as
	// a footer, not another option.
	navSep := c.sty.Border.Render("├" + strings.Repeat(styles.Horizontal, barW-2) + "┤")

	padTo := func(line string) string {
		return line + strings.Repeat(" ", max(0, barW-lipgloss.Width(line)))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", titleBar)
	for _, line := range descLines {
		fmt.Fprintf(&b, "%s\n", padTo(c.sty.Base.Render(line)))
	}
	if truncated {
		fmt.Fprintf(&b, "%s\n", padTo(c.sty.Muted.Render("  ... (truncated)")))
	}
	fmt.Fprintf(&b, "%s\n", sep)
	// Risk level tag on its own line above the options.
	fmt.Fprintf(&b, "%s\n", padTo("  "+levelTag))
	// Options stacked vertically; the selected one is highlighted.
	for i, opt := range opts {
		var row string
		if i == c.selected {
			row = c.sty.Primary.Bold(true).
				Background(lipgloss.Color(styles.BtnYesBg)).
				Padding(0, 2).
				Render("▸ " + opt.Label)
			row = "  " + row
		} else {
			row = c.sty.Muted.Render("    " + opt.Label)
		}
		fmt.Fprintf(&b, "%s\n", padTo(row))
	}
	// Nav hint footer, visually separated from the options.
	fmt.Fprintf(&b, "%s\n", navSep)
	fmt.Fprintf(&b, "%s\n", padTo(c.sty.Muted.Render("  ↑↓选择  Enter确认  Esc拒绝")))
	b.WriteString(bottomBorder)

	return b.String()
}

func (c *ConfirmBar) descLines(maxW int) []string {
	desc := c.formatDesc()
	if desc == "" {
		return nil
	}
	return wrapText("  "+desc, maxW)
}

func (c *ConfirmBar) formatDesc() string {
	if c.isPermissionConfirm() {
		return c.formatPermissionDesc()
	}
	switch c.req.ToolName {
	case "bash":
		if cmd, ok := c.req.Args["command"].(string); ok && cmd != "" {
			return common.FormatCommandPreview(cmd, 600)
		}
	case "write":
		if p, ok := c.req.Args["path"].(string); ok && p != "" {
			return "write " + p
		}
	case "edit":
		if p, ok := c.req.Args["path"].(string); ok && p != "" {
			return "edit " + p
		}
	case "/plugin install":
		if summary, ok := c.req.Args["summary"].(string); ok && summary != "" {
			return summary
		}
		return "Install plugin from " + fmt.Sprint(c.req.Args["source"])
	}
	if p, ok := c.req.Args["path"].(string); ok && p != "" {
		return c.req.ToolName + " " + p
	}
	return c.req.ToolName
}

func (c *ConfirmBar) isPermissionConfirm() bool {
	_, ok := c.req.Args["permission_reason"]
	return ok
}

func (c *ConfirmBar) formatPermissionDesc() string {
	var lines []string
	if cmd, ok := c.req.Args["command"].(string); ok && cmd != "" {
		lines = append(lines, common.FormatCommandPreview(cmd, 600))
	} else {
		lines = append(lines, c.req.ToolName)
	}
	if reason, ok := c.req.Args["permission_reason"].(string); ok && reason != "" {
		lines = append(lines, "Permission: "+reason)
	}
	if caps, ok := c.req.Args["permission_capabilities"].(string); ok && caps != "" {
		lines = append(lines, "Capabilities: "+caps)
	}
	if scope, ok := c.req.Args["permission_scope"].(string); ok && scope != "" {
		if scope == "project" {
			lines = append(lines, "Scope: project (can be remembered for this workspace)")
		} else {
			lines = append(lines, "Scope: "+scope)
		}
	}
	if workspace, ok := c.req.Args["workspace"].(string); ok && workspace != "" {
		lines = append(lines, "Workspace: "+workspace)
	}
	if class, ok := c.req.Args["commandClass"].(string); ok && class != "" {
		lines = append(lines, "Command class: "+class)
	}
	return strings.Join(lines, "\n")
}

func wrapText(text string, maxW int) []string {
	if maxW <= 0 {
		return []string{text}
	}
	var lines []string
	remaining := []rune(text)
	for len(remaining) > 0 {
		displayW := lipgloss.Width(string(remaining))
		if displayW <= maxW && !strings.ContainsRune(string(remaining), '\n') {
			lines = append(lines, strings.TrimRight(string(remaining), "\n"))
			break
		}
		cut := 0
		w := 0
		lastSpace := -1
		for i, r := range remaining {
			if r == '\n' {
				lines = append(lines, string(remaining[:i]))
				remaining = remaining[i+1:]
				cut = -1
				break
			}
			rw := lipgloss.Width(string(r))
			if w+rw > maxW {
				break
			}
			w += rw
			cut = i + 1
			if r == ' ' {
				lastSpace = i
			}
		}
		if cut < 0 {
			continue
		}
		if lastSpace > 0 && lastSpace < cut {
			cut = lastSpace
		}
		if cut == 0 {
			cut = 1
		}
		lines = append(lines, strings.TrimRight(string(remaining[:cut]), " "))
		remaining = remaining[cut:]
		if len(remaining) > 0 && remaining[0] == ' ' {
			remaining = remaining[1:]
		}
	}
	return lines
}
