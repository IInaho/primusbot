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

func TestAnalyzeCommandParsesLiteralCommandSubstitution(t *testing.T) {
	plan := analyzeCommand("echo $(pwd)", "/repo")
	if plan.Unknown {
		t.Fatal("literal command substitution should be analyzed, not marked unknown")
	}
	if plan.CommandClass != "read-only" {
		t.Fatalf("CommandClass = %q, want read-only", plan.CommandClass)
	}
}

func TestAnalyzeCommandMarksDynamicCommandNameUnknown(t *testing.T) {
	plan := analyzeCommand("echo $($RUNNER)", "/repo")
	if !plan.Unknown {
		t.Fatal("dynamic command name should be unknown")
	}
}
