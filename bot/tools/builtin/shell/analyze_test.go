package shell

import "testing"

func TestAnalyzeCommandDetectsNetworkAndCache(t *testing.T) {
	plan := analyzeCommand("npm install", "/repo")
	if plan.CommandClass != "package-install" {
		t.Fatalf("CommandClass = %q, want package-install", plan.CommandClass)
	}
	if !plan.NeedsNetwork {
		t.Fatal("npm install should require network")
	}
	if len(plan.CachePaths) == 0 {
		t.Fatal("npm install should request cache paths")
	}
}

func TestAnalyzeCommandMarksDynamicShellUnknown(t *testing.T) {
	plan := analyzeCommand("echo $(whoami)", "/repo")
	if !plan.Unknown {
		t.Fatal("command substitution should be unknown")
	}
	if plan.CommandClass != "unknown" {
		t.Fatalf("CommandClass = %q, want unknown", plan.CommandClass)
	}
}
