package components

import (
	"fmt"
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	charLimit     = 32768
	maxInputLines = 8
	promptCols    = 2
)

type Input struct {
	textarea        textarea.Model
	width           int
	follow          bool
	permissionMode  string
	model           string
	reasoningEffort string
	sending         bool
	history         []string
	historyIdx      int
	savedInput      string
	historyActive   bool
}

func NewInput(width int) *Input {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.SetVirtualCursor(false)
	ta.Focus()
	ta.CharLimit = charLimit
	ta.MaxHeight = maxInputLines
	ta.MinHeight = 1
	ta.DynamicHeight = true
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))

	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Focused.Placeholder = styles.MutedStyle
	s.Blurred.Placeholder = styles.MutedStyle
	ta.SetStyles(s)

	prompt := styles.CatEyeStyle.Bold(true).Render("┃ ")
	ta.SetPromptFunc(promptCols, func(info textarea.PromptInfo) string { return prompt })
	ta.SetWidth(max(1, width))

	return &Input{textarea: ta, width: width, follow: true}
}

func (i *Input) SetWidth(width int) { i.width = width; i.textarea.SetWidth(max(1, width)) }
func (i *Input) Width() int         { return i.width }

func (i *Input) Value() string     { return strings.TrimRight(i.textarea.Value(), "\n\t\r ") }
func (i *Input) SetValue(v string) { i.textarea.SetValue(v) }
func (i *Input) SetCursorEnd()     { i.textarea.MoveToEnd() }

func (i *Input) Reset() {
	i.textarea.Reset()
	i.sending = false
	i.historyActive = false
}

func (i *Input) AddHistory(entry string) {
	if entry == "" {
		return
	}
	if len(i.history) > 0 && i.history[len(i.history)-1] == entry {
		return
	}
	i.history = append(i.history, entry)
	i.historyIdx = len(i.history)
}

func (i *Input) SetHistory(entries []string) {
	i.history = append(i.history[:0], entries...)
	i.historyIdx = len(i.history)
	i.historyActive = false
	i.savedInput = ""
}

func (i *Input) History() []string {
	out := make([]string, len(i.history))
	copy(out, i.history)
	return out
}

func (i *Input) HistoryUp() {
	if len(i.history) == 0 {
		return
	}
	if i.historyIdx == len(i.history) {
		i.savedInput = i.textarea.Value()
	}
	if i.historyIdx > 0 {
		i.historyIdx--
		i.SetValue(i.history[i.historyIdx])
	}
	i.historyActive = true
}

func (i *Input) HistoryDown() {
	if i.historyIdx >= len(i.history) {
		return
	}
	i.historyIdx++
	if i.historyIdx == len(i.history) {
		i.SetValue(i.savedInput)
		i.historyActive = false
	} else {
		i.SetValue(i.history[i.historyIdx])
	}
}

func (i *Input) SetSending(sending bool) {
	i.sending = sending
	var text string
	if sending {
		text = styles.MutedStyle.Render("⋯ ")
	} else {
		text = styles.CatEyeStyle.Bold(true).Render("┃ ")
	}
	i.textarea.SetPromptFunc(promptCols, func(info textarea.PromptInfo) string { return text })
}

func (i *Input) SetFollow(follow bool) { i.follow = follow }

// SetPermissionMode sets the permission mode shown next to Follow in the
// footer ("manual"/"full"; empty hides the field).
func (i *Input) SetPermissionMode(mode string) { i.permissionMode = mode }

// SetModel sets the current model shown in the footer after Perm.
// Empty hides the field.
func (i *Input) SetModel(model string) { i.model = model }

// SetReasoningEffort sets the configured model effort shown after Model.
// Empty means the provider/model default and is rendered as Auto.
func (i *Input) SetReasoningEffort(effort string) { i.reasoningEffort = strings.TrimSpace(effort) }

func (i *Input) CanCursorUp() bool {
	return i.textarea.Line() > 0 || i.textarea.LineInfo().RowOffset > 0
}

func (i *Input) CanCursorDown() bool {
	info := i.textarea.LineInfo()
	return i.textarea.Line() < i.textarea.LineCount()-1 || info.RowOffset < info.Height-1
}

func (i *Input) Height() int { return 4 + i.textarea.Height() }

func (i *Input) Cursor() *tea.Cursor {
	c := i.textarea.Cursor()
	if c == nil {
		return nil
	}
	return tea.NewCursor(c.X, c.Y+1)
}

func (i *Input) Update(msg tea.Msg) (*Input, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		if m.String() == "enter" {
			return i, nil
		}
	}
	var cmd tea.Cmd
	i.textarea, cmd = i.textarea.Update(msg)
	return i, cmd
}

func (i *Input) View() string {
	w := max(20, i.width)
	line := styles.BorderStyle.Render(strings.Repeat(styles.Horizontal, w))

	tv := strings.TrimRight(i.textarea.View(), "\n")

	txt := "Auto"
	if !i.follow {
		txt = "Manual"
	}
	prefix := styles.BorderStyle.Render(styles.Vertical + " ")
	footer := prefix +
		styles.SubtleStyle.Render("Follow:") + " " +
		styles.TealStyle.Render(txt)
	permission := "Manual"
	permissionStyle := styles.TealStyle
	switch i.permissionMode {
	case "full":
		permission = "FULL"
		permissionStyle = styles.RedStyle.Bold(true)
		footer += styles.SubtleStyle.Render(" · Perm: ") + permissionStyle.Render(permission)
	case "manual", "":
		footer += styles.SubtleStyle.Render(" · Perm: ") + permissionStyle.Render(permission)
	default:
		permission = i.permissionMode
		permissionStyle = styles.MutedStyle
		footer += styles.SubtleStyle.Render(" · Perm: ") + permissionStyle.Render(permission)
	}
	if i.model != "" {
		effort := i.displayReasoningEffort()
		footer += styles.SubtleStyle.Render(" · Effort: ") + styles.YellowStyle.Render(effort)
		footer += styles.SubtleStyle.Render(" · Model: ") + styles.CatEyeStyle.Render(i.model)
	}

	if lipgloss.Width(footer) > w-1 {
		compact := prefix + styles.SubtleStyle.Render("F:") + styles.TealStyle.Render(txt) +
			styles.SubtleStyle.Render(" · P:") + permissionStyle.Render(permission)
		if i.model != "" {
			compact += styles.SubtleStyle.Render(" · E:") + styles.YellowStyle.Render(i.displayReasoningEffort()) +
				styles.SubtleStyle.Render(" · ") + styles.CatEyeStyle.Render(i.model)
		}
		footer = compact
		if lipgloss.Width(footer) > w-1 {
			footer = prefix + styles.SubtleStyle.Render("E:") + styles.YellowStyle.Render(i.displayReasoningEffort()) +
				styles.SubtleStyle.Render(" · F:") + styles.TealStyle.Render(txt) +
				styles.SubtleStyle.Render(" · P:") + permissionStyle.Render(permission) +
				styles.SubtleStyle.Render(" · ") + styles.CatEyeStyle.Render(i.model)
		}
	}
	footer = ansi.Truncate(footer, max(1, w-1), "")
	pad := max(0, w-lipgloss.Width(footer)-1)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", line)
	fmt.Fprintf(&b, "%s\n\n", tv)
	fmt.Fprintf(&b, "%s%s%s\n", footer, strings.Repeat(" ", pad), styles.BorderStyle.Render(styles.Vertical))
	b.WriteString(line)
	return b.String()
}

func (i *Input) displayReasoningEffort() string {
	if i.reasoningEffort == "" {
		return "Auto"
	}
	return i.reasoningEffort
}

func (i *Input) Init() tea.Cmd { return tea.Batch(textarea.Blink, BlinkTick()) }
