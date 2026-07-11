package report

import (
	"fmt"
	"strings"

	"nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
	"nekocode/util/text"

	"charm.land/lipgloss/v2"
)

type Report struct {
	Budget          int
	SystemPrompt    int
	TodoText        int
	SkillList       int
	ToolDefTokens   int
	Messages        int
	Archived        int
	ClearedMarkers  int
	CompactCount    int
	TrimCount       int
	ToolDefCount    int
	UserMessages    int
	SysInjections   int
	AssistantMsgs   int
	ToolResults     int
	CacheHitTokens  int
	CacheMissTokens int
	CacheHitRatio   float64
	SubCount        int
	SubTokens       int
	SubCacheHit     int
	SubCacheMiss    int
}

type Builder struct {
	State *state.State
}

func (rb *Builder) Report() Report {
	rb.State.Mu.RLock()
	defer rb.State.Mu.RUnlock()

	r := Report{}
	r.SystemPrompt = token.EstimateString(rb.State.Ctx.SystemPrompt)
	r.TodoText = token.EstimateString(rb.State.Ctx.Todo)
	r.SkillList = token.EstimateString(rb.State.Ctx.Skills)

	for i := rb.State.Ctx.CompactBoundary; i < len(rb.State.Ctx.Messages); i++ {
		msg := rb.State.Ctx.Messages[i]
		if msg.Content == compression.ClearedMarker {
			r.ClearedMarkers++
			continue
		}
		switch msg.Role {
		case "user":
			if msg.Source == "system" {
				r.SysInjections++
			} else {
				r.UserMessages++
			}
		case "assistant":
			r.AssistantMsgs++
		case "tool":
			r.ToolResults++
		}
	}
	r.Messages = token.EstimateTokens(rb.State.Ctx.Messages[rb.State.Ctx.CompactBoundary:])
	r.Archived = rb.State.Ctx.CompactBoundary
	r.CompactCount = rb.State.CompactCount
	r.TrimCount = rb.State.TrimCount
	r.Budget = rb.State.ContextWindow
	r.CacheHitTokens, r.CacheMissTokens = rb.State.Tracker.CacheStats()
	r.CacheHitRatio = rb.State.Tracker.CacheHitRatio()
	sub := rb.State.Tracker.SubStats()
	r.SubCount = sub.Count
	r.SubTokens = sub.TotalTokens
	r.SubCacheHit = sub.CacheHitTokens
	r.SubCacheMiss = sub.CacheMissTokens
	return r
}

func Format(r Report) string {
	used := r.SystemPrompt + r.ToolDefTokens + r.TodoText + r.SkillList + r.Messages
	free := r.Budget - used
	if free < 0 {
		free = 0
	}
	pct := func(n int) string {
		if r.Budget == 0 {
			return ""
		}
		return fmt.Sprintf("(%.0f%%)", float64(n)/float64(r.Budget)*100)
	}
	item := func(ch, label string, n int) string {
		return barColors[ch].Render(barChars[ch]) + " " + label + ": " + text.FormatTokens(n) + " " + barColors["free"].Render(pct(n))
	}

	bar := BuildBar(r.Budget, []BarSegment{
		{Size: r.SystemPrompt, Kind: "sys"},
		{Size: r.ToolDefTokens + r.TodoText, Kind: "tools"},
		{Size: r.SkillList, Kind: "skills"},
		{Size: r.Messages, Kind: "msgs"},
		{Size: free, Kind: "free"},
	}, 20)

	s := fmt.Sprintf("%s  %s / %s\n\n%s  %s\n%s  %s\n\n%s",
		bar, text.FormatTokens(used), text.FormatTokens(r.Budget),
		item("sys", "System", r.SystemPrompt),
		item("tools", "Tools", r.ToolDefTokens),
		item("msgs", "Messages", r.Messages),
		item("skills", "Skills", r.SkillList),
		barColors["free"].Render(fmt.Sprintf("%d tools · %d msgs · %d archived  %s Free: %s",
			r.ToolDefCount, r.UserMessages+r.AssistantMsgs+r.ToolResults, r.Archived,
			text.FormatTokens(free), pct(free))),
	)

	if r.CacheHitTokens > 0 || r.CacheMissTokens > 0 {
		hit := text.FormatTokens(r.CacheHitTokens)
		miss := text.FormatTokens(r.CacheMissTokens)
		ratio := fmt.Sprintf("%.0f%%", r.CacheHitRatio*100)
		s += fmt.Sprintf("\n%s Cache: hit %s / miss %s · %s",
			barColors["cache"].Render(barChars["cache"]), hit, miss, ratio)
	}

	if r.SubCount > 0 {
		subTok := text.FormatTokens(r.SubTokens)
		subHit := text.FormatTokens(r.SubCacheHit)
		subMiss := text.FormatTokens(r.SubCacheMiss)
		var subRatio string
		if total := r.SubCacheHit + r.SubCacheMiss; total > 0 {
			subRatio = fmt.Sprintf(" · hit %.0f%%", float64(r.SubCacheHit)/float64(total)*100)
		}
		s += fmt.Sprintf("\n%s Subagents: %d runs · %s tokens · hit %s / miss %s%s",
			barColors["sub"].Render(barChars["sub"]), r.SubCount, subTok, subHit, subMiss, subRatio)
	}
	return s
}

type BarSegment struct {
	Size int
	Kind string
}

var barColors = map[string]lipgloss.Style{
	"sys":    lipgloss.NewStyle().Foreground(lipgloss.Color("#888")),
	"tools":  lipgloss.NewStyle().Foreground(lipgloss.Color("#999")),
	"todo":   lipgloss.NewStyle().Foreground(lipgloss.Color("#d47757")),
	"skills": lipgloss.NewStyle().Foreground(lipgloss.Color("#ffc107")),
	"msgs":   lipgloss.NewStyle().Foreground(lipgloss.Color("#9334ea")),
	"free":   lipgloss.NewStyle().Foreground(lipgloss.Color("#666")),
	"cache":  lipgloss.NewStyle().Foreground(lipgloss.Color("#6ab")),
	"sub":    lipgloss.NewStyle().Foreground(lipgloss.Color("#6ab")),
}

var barChars = map[string]string{
	"sys": "⛁", "tools": "⛁", "todo": "⛀", "skills": "⛀", "msgs": "⛁", "free": "⛶",
	"cache": "⛂", "sub": "⛃",
}

func BuildBar(total int, segments []BarSegment, width int) string {
	if total <= 0 {
		return ""
	}
	allocated := make([]int, len(segments))
	remaining := width
	for i, s := range segments {
		if s.Size > 0 {
			w := s.Size * width / total
			if w < 1 {
				w = 1
			}
			allocated[i] = w
			remaining -= w
		}
	}
	for i := len(segments) - 1; i >= 0 && remaining > 0; i-- {
		if segments[i].Size > 0 {
			allocated[i] += remaining
			break
		}
	}

	var b strings.Builder
	for i, s := range segments {
		if allocated[i] <= 0 {
			continue
		}
		ch := barChars[s.Kind]
		if ch == "" {
			ch = " "
		}
		sty := barColors[s.Kind]
		for range allocated[i] {
			fmt.Fprintf(&b, "%s ", sty.Render(ch))
		}
	}
	return b.String()
}
