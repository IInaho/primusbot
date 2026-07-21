package artifact

import "testing"

func TestClassifyToolOutput(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		content   string
		wantKind  Kind
		wantExt   string
		wantMatch bool
	}{
		{
			name:      "patch tool name wins",
			toolName:  "apply_patch",
			content:   "*** Begin Patch\n*** Update File: README.md\n@@\n-old\n+new\n*** End Patch",
			wantKind:  KindPatch,
			wantExt:   ".patch",
			wantMatch: true,
		},
		{
			name:      "review tool name",
			toolName:  "security_review",
			content:   "Findings\nSeverity: high\nRisk: missing guard",
			wantKind:  KindReview,
			wantExt:   ".md",
			wantMatch: true,
		},
		{
			name:      "diff content",
			toolName:  "unknown",
			content:   "header\n--- a/file\n+++ b/file\n@@\n-old\n+new",
			wantKind:  KindDiff,
			wantExt:   ".patch",
			wantMatch: true,
		},
		{
			name:      "empty content",
			toolName:  "diff",
			content:   "  ",
			wantMatch: false,
		},
		{
			name:      "plain output",
			toolName:  "shell",
			content:   "ok",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ClassifyToolOutput(tt.toolName, tt.content)
			if ok != tt.wantMatch {
				t.Fatalf("match = %v, want %v", ok, tt.wantMatch)
			}
			if !ok {
				return
			}
			if got.Kind != tt.wantKind || got.Extension != tt.wantExt {
				t.Fatalf("classification = %#v, want kind=%q ext=%q", got, tt.wantKind, tt.wantExt)
			}
		})
	}
}
