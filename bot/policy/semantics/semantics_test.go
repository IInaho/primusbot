package semantics

import "testing"

func TestClassifyBashExplorationAndVerification(t *testing.T) {
	sem := ClassifyToolCall("bash", map[string]any{"command": "cat README.md"})
	if !sem.Exploratory || !sem.SourceProducing {
		t.Fatalf("cat should be exploratory source-producing: %+v", sem)
	}

	sem = ClassifyToolCall("bash", map[string]any{"command": "go test ./..."})
	if !sem.Verifying || sem.Exploratory {
		t.Fatalf("go test should be verifying, not exploratory: %+v", sem)
	}
}

func TestClassifyBashVerificationIsNotMutation(t *testing.T) {
	for _, cmd := range []string{
		"go test ./...",
		"make test",
		"npm run lint",
		"npm run test:unit",
		"npx tsc --noEmit",
		"npm run typecheck",
		"pnpm build",
		"cargo check",
		"python -m pytest",
		"just check",
		"task test",
		"timeout --foreground 30 pytest",
		"timeout -k 5 30 pytest",
		"timeout --kill-after=5 30 pytest",
		"env GOCACHE=/tmp/go-build go test ./...",
		"env -u GOCACHE go test ./...",
		"env -C . GOCACHE=/tmp/go-build go test ./...",
		"env -- go test ./...",
		"cd gui && npm run typecheck",
		`go test "$PKG"`,
	} {
		sem := ClassifyToolCall("bash", map[string]any{"command": cmd})
		if !sem.Verifying || sem.Mutating {
			t.Fatalf("%q should be verifying without mutating: %+v", cmd, sem)
		}
	}
}

func TestClassifyBashVerificationWithSeparateMutation(t *testing.T) {
	for _, cmd := range []string{
		"go test ./... > test.out",
		"go test ./... && rm test.out",
		"npm run test && touch marker",
	} {
		sem := ClassifyToolCall("bash", map[string]any{"command": cmd})
		if !sem.Verifying || !sem.Mutating {
			t.Fatalf("%q should be both verifying and mutating: %+v", cmd, sem)
		}
	}
}

func TestClassifyBashVerificationTrust(t *testing.T) {
	for _, cmd := range []string{"go test ./...", "pytest", "env GOCACHE=/tmp/go-build go test ./...", "npx tsc --noEmit"} {
		sem := ClassifyToolCall("bash", map[string]any{"command": cmd})
		if !sem.Verifying || !sem.VerificationTrusted || sem.VerificationProjectRule {
			t.Fatalf("%q trust flags = %+v, want trusted verification", cmd, sem)
		}
	}

	for _, cmd := range []string{"npm run test", "make test", "just check", "cd gui && npm run typecheck"} {
		sem := ClassifyToolCall("bash", map[string]any{"command": cmd})
		if !sem.Verifying || sem.VerificationTrusted || !sem.VerificationProjectRule {
			t.Fatalf("%q trust flags = %+v, want project-rule verification", cmd, sem)
		}
	}
}

func TestClassifyBashVerificationDoesNotMatchArbitraryScripts(t *testing.T) {
	for _, cmd := range []string{"npm run deploy", "just release", "make clean", `npm run "$SCRIPT"`} {
		sem := ClassifyToolCall("bash", map[string]any{"command": cmd})
		if sem.Verifying {
			t.Fatalf("%q should not be classified as verifying: %+v", cmd, sem)
		}
	}
}

func TestClassifyMutation(t *testing.T) {
	for _, name := range []string{"write", "edit"} {
		sem := ClassifyToolCall(name, nil)
		if !sem.Mutating {
			t.Fatalf("%s should be mutating: %+v", name, sem)
		}
	}

	for _, cmd := range []string{"echo hi > out.txt", "make clean", "go test ./... | tee test.out", "sed -i 's/a/b/' main.go", "perl -pi -e 's/a/b/' main.go"} {
		sem := ClassifyToolCall("bash", map[string]any{"command": cmd})
		if !sem.Mutating {
			t.Fatalf("%q should be mutating: %+v", cmd, sem)
		}
	}

	sem := ClassifyToolCall("bash", map[string]any{"command": "perl -p -e 's/a/b/' main.go"})
	if sem.Mutating {
		t.Fatalf("perl without -i should not be mutating: %+v", sem)
	}
}
