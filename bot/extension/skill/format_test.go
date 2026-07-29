package skill

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildSkillListText(t *testing.T) {
	skills := []*Skill{
		{Name: "deploy", Description: "deploy app when deploying"},
		{Name: "review", Description: "review code"},
	}

	text := buildSkillListText(skills, nil, 64000)
	if text == "" || !strings.Contains(text, "deploy") || !strings.Contains(text, "review") {
		t.Error("missing skill names")
	}
	if !strings.Contains(text, "when deploying") {
		t.Error("missing description trigger guidance")
	}

	// Loaded filtering.
	text = buildSkillListText(skills, map[string]bool{"deploy": true}, 64000)
	if strings.Contains(text, "deploy") {
		t.Error("loaded skill should be excluded")
	}

	// All loaded.
	if buildSkillListText(skills, map[string]bool{"deploy": true, "review": true}, 64000) != "" {
		t.Error("expected empty when all loaded")
	}

	// Edge cases.
	if buildSkillListText(nil, nil, 0) != "" {
		t.Error("nil skills should return empty")
	}
	if buildSkillListText([]*Skill{}, nil, 0) != "" {
		t.Error("empty skills should return empty")
	}
}

func TestBuildSkillListTextTruncation(t *testing.T) {
	var skills []*Skill
	for i := 0; i < 200; i++ {
		skills = append(skills, &Skill{Name: fmt.Sprintf("s%03d", i), Description: "desc"})
	}

	// 64000/100 = 640 chars budget; far less than 200 entries need.
	text := buildSkillListText(skills, nil, 64000)
	if !strings.Contains(text, "s000") {
		t.Error("first entry should always be listed, even over budget")
	}
	if !strings.Contains(text, "omitted due to token budget") {
		t.Error("expected truncation notice")
	}
	if strings.Contains(text, "s199") {
		t.Error("entries past the budget should be omitted")
	}
}

func TestFormatForContext(t *testing.T) {
	sk := &Skill{
		Name: "deploy", Content: "# Deploy\n\nbuild",
		Dir: "/tmp/skills/deploy", Files: []string{"script.sh"},
	}
	text := FormatForContext(sk)
	if !strings.Contains(text, `<skill_content name="deploy">`) {
		t.Error("missing tag")
	}
	if !strings.Contains(text, "# Deploy") {
		t.Error("missing body")
	}
	if !strings.Contains(text, "script.sh") {
		t.Error("missing file")
	}
}
