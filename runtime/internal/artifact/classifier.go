package artifact

import "strings"

type Kind string

const (
	KindDiff   Kind = "diff"
	KindPatch  Kind = "patch"
	KindReview Kind = "review"
)

type Classification struct {
	Kind      Kind
	Extension string
}

func ClassifyToolOutput(toolName, content string) (Classification, bool) {
	if strings.TrimSpace(content) == "" {
		return Classification{}, false
	}
	switch {
	case isPatchArtifact(toolName, content):
		return Classification{Kind: KindPatch, Extension: ".patch"}, true
	case isReviewArtifact(toolName, content):
		return Classification{Kind: KindReview, Extension: ".md"}, true
	case isDiffArtifact(toolName, content):
		return Classification{Kind: KindDiff, Extension: ".patch"}, true
	default:
		return Classification{}, false
	}
}

func isPatchArtifact(toolName, content string) bool {
	name := strings.ToLower(toolName)
	if name == "patch" || name == "apply_patch" {
		return true
	}
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "*** Begin Patch") && strings.Contains(trimmed, "*** End Patch")
}

func isReviewArtifact(toolName, content string) bool {
	name := strings.ToLower(toolName)
	if name == "review" || name == "code_review" || name == "security_review" || name == "architecture_review" {
		return true
	}
	lower := strings.ToLower(content)
	return strings.Contains(lower, "findings") && (strings.Contains(lower, "severity") || strings.Contains(lower, "risk"))
}

func isDiffArtifact(toolName, content string) bool {
	name := strings.ToLower(toolName)
	if name == "edit" || name == "write" || name == "diff" {
		return true
	}
	return strings.Contains(content, "\n---") && strings.Contains(content, "\n+++")
}
