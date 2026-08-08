// Splash 启动页：ASCII 猫 + 猫眼闪烁动画（blinkCount 驱动）和标题。
// 按 Enter 进入聊天界面。
package components

import (
	"fmt"
	"strings"
	"time"

	"nekocode/interaction/tui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TickMsg 驱动 Splash 猫眼闪烁的定时消息。
type TickMsg struct{}

// BlinkTick 每 500ms 产生一个 TickMsg。
func BlinkTick() tea.Cmd {
	return tea.Every(time.Millisecond*500, func(t time.Time) tea.Msg { return TickMsg{} })
}

const inputReserved = 7 // Input.Height()=5 + 2 separator lines in View()

type Splash struct {
	width  int
	height int
	blink  bool
}

func NewSplash(width, height int, _ string) *Splash {
	return &Splash{width: width, height: height}
}

func (s *Splash) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Splash) Blink() {
	s.blink = !s.blink
}

func (s *Splash) View() string {
	w := max(60, s.width)
	h := max(20, s.height)

	center := lipgloss.NewStyle().Width(w).Align(lipgloss.Center)
	cat := s.renderCat()
	title := s.renderTitle()
	sep := s.renderSeparator()
	subtitle := s.renderSubtitle()

	var lines []string

	// Cat: block-center to preserve internal structure.
	catLines := strings.Split(cat, "\n")
	maxCatW := 0
	for _, l := range catLines {
		if cw := lipgloss.Width(l); cw > maxCatW {
			maxCatW = cw
		}
	}
	catPad := max(0, (w-maxCatW)/2)
	for _, l := range catLines {
		lines = append(lines, strings.Repeat(" ", catPad)+l)
	}

	for line := range strings.SplitSeq(title, "\n") {
		lines = append(lines, center.Render(line))
	}
	lines = append(lines, center.Render(sep))
	lines = append(lines, center.Render(subtitle))

	contentBlock := strings.Join(lines, "\n")
	contentH := len(lines)

	// Input.Height()=5 + 2 separator lines in tui.go View().
	reserved := inputReserved
	topPad := max(0, (h-reserved-contentH)/2)

	var b strings.Builder
	for range topPad {
		b.WriteString("\n")
	}
	b.WriteString(contentBlock)
	return b.String()
}

func (s *Splash) renderCat() string {
	// Black cat with glowing teal eyes
	//
	//      /\___/\
	//     ( o   o )

	body := styles.CatBodyStyle
	eyeStyle := styles.CatEyeStyle
	if s.blink {
		eyeStyle = styles.SubtleStyle // dim eyes on blink, keeps width constant
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", body.Render(" /\\___/\\"))
	fmt.Fprintf(&b, "%s%s%s%s%s\n", body.Render("( "), eyeStyle.Render("o"), body.Render(" . "), eyeStyle.Render("o"), body.Render(" )"))

	return b.String()
}

func (s *Splash) renderTitle() string {
	return styles.PrimaryStyle.Bold(true).Render("N E K O C O D E")
}

func (s *Splash) renderSeparator() string {
	seg := strings.Repeat(styles.Horizontal, 12)
	return styles.MutedStyle.Render(seg) + styles.PrimaryStyle.Render(" ◆ ") + styles.MutedStyle.Render(seg)
}

func (s *Splash) renderSubtitle() string {
	return styles.MutedStyle.Render("Ready to chat  >^.^<")
}
