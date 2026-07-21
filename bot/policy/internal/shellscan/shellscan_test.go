package shellscan

import (
	"reflect"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

// These tests pin the exact extraction behavior previously implemented
// separately in bot/policy/ledger and bot/policy/semantics; both packages now
// rely on this shared scan.

func TestScanShellCalls(t *testing.T) {
	cases := []struct {
		cmd   string
		calls [][]string
	}{
		{"cat bot/policy/ledger/ledger.go", [][]string{{"cat", "bot/policy/ledger/ledger.go"}}},
		{"cd bot && cat policy/ledger/ledger.go", [][]string{{"cd", "bot"}, {"cat", "policy/ledger/ledger.go"}}},
		{"go test ./... | tee test.out", [][]string{{"go", "test", "./..."}, {"tee", "test.out"}}},
		{"go test ./...&&touch marker.txt", [][]string{{"go", "test", "./..."}, {"touch", "marker.txt"}}},
		// Non-literal arguments after the command word contribute "".
		{`go test "$PKG"`, [][]string{{"go", "test", ""}}},
		{`npm run "$SCRIPT"`, [][]string{{"npm", "run", ""}}},
		// A non-literal command word skips the call but nested calls are walked.
		{"$(echo foo) bar", [][]string{{"echo", "foo"}}},
	}
	for _, c := range cases {
		scan := ScanShell(c.cmd)
		if !scan.OK {
			t.Fatalf("%q: expected parse success", c.cmd)
		}
		if !reflect.DeepEqual(scan.Calls, c.calls) {
			t.Fatalf("%q calls = %#v, want %#v", c.cmd, scan.Calls, c.calls)
		}
	}
}

func TestScanShellRedirects(t *testing.T) {
	cases := []struct {
		cmd      string
		hasWrite bool
		targets  []string
	}{
		{"go test ./... > test.out", true, []string{"test.out"}},
		{"go test ./...>test.out", true, []string{"test.out"}},
		{"go test ./... >| test.out", true, []string{"test.out"}},
		{"cmd >> log.txt", true, []string{"log.txt"}},
		{"cmd 2> err.txt", true, []string{"err.txt"}},
		{"cmd &> all.txt", true, []string{"all.txt"}},
		{"cmd &>> all.txt", true, []string{"all.txt"}},
		{"cmd > /dev/null", true, []string{"/dev/null"}},
		{"cmd < in.txt", false, nil},
		{"cat README.md", false, nil},
		// Non-literal redirect targets set the flag but yield no path.
		{`cmd > "$OUT"`, true, nil},
	}
	for _, c := range cases {
		scan := ScanShell(c.cmd)
		if !scan.OK {
			t.Fatalf("%q: expected parse success", c.cmd)
		}
		if scan.HasWriteRedirect != c.hasWrite {
			t.Fatalf("%q HasWriteRedirect = %v, want %v", c.cmd, scan.HasWriteRedirect, c.hasWrite)
		}
		if !reflect.DeepEqual(scan.RedirectTargets, c.targets) {
			t.Fatalf("%q RedirectTargets = %#v, want %#v", c.cmd, scan.RedirectTargets, c.targets)
		}
	}
}

func TestScanShellParseFailure(t *testing.T) {
	scan := ScanShell(`echo "unterminated`)
	if scan.OK {
		t.Fatal("unterminated quote should fail parsing")
	}
	if scan.Calls != nil || scan.HasWriteRedirect || scan.RedirectTargets != nil {
		t.Fatalf("failed parse must return zero scan, got %+v", scan)
	}
}

func TestFields(t *testing.T) {
	cases := []struct {
		in     string
		fields []string
	}{
		{"cat a.go b.go", []string{"cat", "a.go", "b.go"}},
		{`cat "a b" c`, []string{"cat", "a b", "c"}},
		{`touch 'x y'`, []string{"touch", "x y"}},
		{`echo a\ b`, []string{"echo", "a b"}},
		{"echo  a\tb", []string{"echo", "a", "b"}},
		{`echo \`, []string{"echo", "\\"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		got := Fields(c.in)
		if !reflect.DeepEqual(got, c.fields) {
			t.Fatalf("Fields(%q) = %#v, want %#v", c.in, got, c.fields)
		}
	}
}

func TestIsMutatingCommand(t *testing.T) {
	for _, name := range []string{"mkdir", "touch", "cp", "mv", "rm", "rmdir", "chmod", "chown"} {
		if !IsMutatingCommand(name) {
			t.Fatalf("%s should be mutating", name)
		}
	}
	for _, name := range []string{"cat", "ls", "git", "tee", "sed", "make", ""} {
		if IsMutatingCommand(name) {
			t.Fatalf("%s should not be mutating", name)
		}
	}
}

func TestIsWriteRedirect(t *testing.T) {
	write := []syntax.RedirOperator{syntax.RdrOut, syntax.AppOut, syntax.ClbOut, syntax.RdrAll, syntax.AppAll}
	for _, op := range write {
		if !IsWriteRedirect(op) {
			t.Fatalf("%v should be a write redirect", op)
		}
	}
	for _, op := range []syntax.RedirOperator{syntax.RdrIn, syntax.RdrInOut, syntax.DplIn, syntax.DplOut, syntax.Hdoc} {
		if IsWriteRedirect(op) {
			t.Fatalf("%v should not be a write redirect", op)
		}
	}
}

func TestLiteralWord(t *testing.T) {
	if got := LiteralWord(nil); got != "" {
		t.Fatalf("nil word = %q, want empty", got)
	}
}
