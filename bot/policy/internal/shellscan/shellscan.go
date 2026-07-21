// Package shellscan provides shared shell command scanning helpers built on
// mvdan.cc/sh, used by the policy ledger and semantics packages so both
// classify the same command the same way.
package shellscan

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Scan is the result of parsing a shell command string.
type Scan struct {
	// OK reports whether the command parsed successfully. When false, all
	// other fields are zero and callers should fall back to string matching.
	OK bool
	// Calls holds the literal fields of every simple command in the input,
	// in walk order. A call whose command word is not a literal is skipped;
	// non-literal arguments after the first contribute an empty string.
	Calls [][]string
	// HasWriteRedirect reports whether any write redirect was found.
	HasWriteRedirect bool
	// RedirectTargets holds the literal target words of write redirects, in
	// walk order. Non-literal or empty targets are omitted.
	RedirectTargets []string
}

// ScanShell parses cmd and extracts literal command calls and write redirects
// in a single AST walk.
func ScanShell(cmd string) Scan {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return Scan{}
	}
	scan := Scan{OK: true}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.Redirect:
			if !IsWriteRedirect(n.Op) {
				return true
			}
			scan.HasWriteRedirect = true
			if p := LiteralWord(n.Word); p != "" {
				scan.RedirectTargets = append(scan.RedirectTargets, p)
			}
		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				return true
			}
			var fields []string
			for i, arg := range n.Args {
				word := LiteralWord(arg)
				if i == 0 && word == "" {
					return true
				}
				fields = append(fields, word)
			}
			scan.Calls = append(scan.Calls, fields)
		}
		return true
	})
	return scan
}

// LiteralWord returns the concatenation of a word's literal parts, or "" if
// the word contains any non-literal part (expansions, substitutions, quotes
// with expansions, ...).
func LiteralWord(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range w.Parts {
		lit, ok := part.(*syntax.Lit)
		if !ok {
			return ""
		}
		b.WriteString(lit.Value)
	}
	return b.String()
}

// IsWriteRedirect reports whether op writes to a file (truncate, append,
// clobber, and the & variants combining stderr).
func IsWriteRedirect(op syntax.RedirOperator) bool {
	return op == syntax.RdrOut || op == syntax.AppOut || op == syntax.ClbOut ||
		op == syntax.RdrAll || op == syntax.AppAll
}

// IsMutatingCommand reports whether name is a well-known path-mutating
// command. Callers should pass a basename (filepath.Base).
func IsMutatingCommand(name string) bool {
	switch name {
	case "mkdir", "touch", "cp", "mv", "rm", "rmdir", "chmod", "chown":
		return true
	default:
		return false
	}
}

// Fields is the fallback tokenizer used when the command does not parse. It
// splits on unquoted whitespace and understands single/double quotes and
// backslash escapes, but not expansions or substitutions.
func Fields(s string) []string {
	var fields []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if b.Len() > 0 {
				fields = append(fields, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	if b.Len() > 0 {
		fields = append(fields, b.String())
	}
	return fields
}
