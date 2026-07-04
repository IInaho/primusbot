package semantics

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type Semantics struct {
	Exploratory             bool
	Mutating                bool
	Verifying               bool
	VerificationTrusted     bool
	VerificationProjectRule bool
	SourceProducing         bool
	Delegating              bool
	Network                 bool
	Risky                   bool
}

func ClassifyToolCall(name string, args map[string]any) Semantics {
	switch name {
	case "read", "grep", "glob", "list", "tree", "project_info":
		return Semantics{Exploratory: true, SourceProducing: true}
	case "web_search", "web_fetch":
		return Semantics{Exploratory: true, SourceProducing: true, Network: true}
	case "write", "edit":
		return Semantics{Mutating: true}
	case "todo_write":
		return Semantics{}
	case "task":
		return Semantics{Delegating: true, Exploratory: taskLooksExploratory(args)}
	case "bash":
		return classifyBash(args)
	default:
		return Semantics{}
	}
}

func taskLooksExploratory(args map[string]any) bool {
	t, _ := args["type"].(string)
	return t == "" || t == "researcher"
}

func classifyBash(args map[string]any) Semantics {
	cmd, _ := args["command"].(string)
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return Semantics{Risky: true}
	}
	scan := parseShell(cmd)
	lower := strings.ToLower(cmd)
	sem := Semantics{}
	if BashLooksExploratory(cmd) {
		sem.Exploratory = true
		sem.SourceProducing = true
	}
	trusted, projectRule := bashVerificationTrust(scan.Calls, scan.OK, lower)
	if trusted || projectRule {
		sem.Verifying = true
		sem.VerificationTrusted = trusted
		sem.VerificationProjectRule = projectRule
		sem.SourceProducing = true
	}
	if bashLooksMutating(scan, lower) {
		sem.Mutating = true
	}
	return sem
}

func BashLooksExploratory(cmd string) bool {
	scan := parseShell(cmd)
	if scan.OK {
		for _, call := range scan.Calls {
			if callLooksExploratory(call) {
				return true
			}
		}
		return false
	}
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	for _, p := range []string{
		"ls", "cat ", "head ", "tail ", "less ", "more ", "wc ",
		"find ", "fd ", "rg ", "grep ", "du ", "df ", "file ", "stat ",
		"pwd", "git status", "git log", "git diff", "git show", "git blame",
	} {
		if cmd == p || strings.HasPrefix(cmd, p) {
			return true
		}
	}
	return false
}

func bashVerificationTrust(calls []shellCall, parsed bool, fallback string) (trusted, projectRule bool) {
	if parsed {
		for _, call := range calls {
			t, p := callVerificationTrust(call)
			trusted = trusted || t
			projectRule = projectRule || p
			if trusted && projectRule {
				return trusted, projectRule
			}
		}
		return trusted, projectRule
	}
	for _, p := range fallbackVerificationPrefixes {
		if fallback == p || strings.HasPrefix(fallback, p+" ") || strings.HasPrefix(fallback, p+" ./") {
			return !fallbackVerificationProjectRules[p], fallbackVerificationProjectRules[p]
		}
	}
	return false, false
}

func bashLooksMutating(scan shellScan, fallback string) bool {
	if scan.OK {
		if scan.HasWriteRedirect {
			return true
		}
		for _, call := range scan.Calls {
			if callLooksVerifying(call) {
				continue
			}
			if callLooksMutating(call) {
				return true
			}
		}
		return false
	}
	trusted, projectRule := bashVerificationTrust(nil, false, fallback)
	if trusted || projectRule {
		return false
	}
	for _, p := range fallbackMutatingPrefixes {
		if fallback == p || strings.HasPrefix(fallback, p) {
			return true
		}
	}
	return strings.Contains(fallback, " > ") || strings.Contains(fallback, " >> ")
}

type shellCall struct {
	Name string
	Args []string
}

type shellScan struct {
	Calls            []shellCall
	HasWriteRedirect bool
	OK               bool
}

func parseShell(cmd string) shellScan {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return shellScan{}
	}
	scan := shellScan{OK: true}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.Redirect:
			if n.Op == syntax.RdrOut || n.Op == syntax.AppOut || n.Op == syntax.ClbOut ||
				n.Op == syntax.RdrAll || n.Op == syntax.AppAll {
				scan.HasWriteRedirect = true
			}
		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				return true
			}
			var words []string
			for i, arg := range n.Args {
				word := literalWord(arg)
				if i == 0 && word == "" {
					return true
				}
				words = append(words, strings.ToLower(word))
			}
			c := shellCall{Name: words[0]}
			if len(words) > 1 {
				c.Args = words[1:]
			}
			scan.Calls = append(scan.Calls, c)
		}
		return true
	})
	return scan
}

func literalWord(w *syntax.Word) string {
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

func callLooksExploratory(call shellCall) bool {
	switch call.Name {
	case "ls", "cat", "head", "tail", "less", "more", "wc",
		"find", "fd", "rg", "grep", "du", "df", "file", "stat", "pwd":
		return true
	case "git":
		return hasFirstArg(call, "status", "log", "diff", "show", "blame")
	default:
		return false
	}
}

func callLooksVerifying(call shellCall) bool {
	trusted, projectRule := callVerificationTrust(call)
	return trusted || projectRule
}

func callVerificationTrust(call shellCall) (trusted, projectRule bool) {
	switch call.Name {
	case "go":
		return hasFirstArg(call, "test", "vet"), false
	case "cargo":
		return hasFirstArg(call, "test", "clippy", "check", "build"), false
	case "pytest", "tox", "ruff", "mypy", "pyright", "tsc":
		return true, false
	case "python", "python3":
		return hasArgSequence(call.Args, "-m", "pytest") || hasArgSequence(call.Args, "-m", "mypy"), false
	case "npm":
		return false, packageRunnerLooksVerifying(call.Args, "run-script")
	case "pnpm", "yarn", "bun":
		return false, packageRunnerLooksVerifying(call.Args)
	case "npx":
		return packageRunnerLooksVerifying(call.Args), false
	case "make", "just", "task":
		return false, hasFirstArg(call, "test", "tests", "lint", "check", "verify", "typecheck", "build", "ci")
	case "mvn":
		return false, hasFirstArg(call, "test", "verify", "check")
	case "gradle", "./gradlew", "gradlew":
		return false, hasAnyArg(call.Args, "test", "check", "build")
	case "timeout", "time", "env":
		return wrappedCallVerificationTrust(call)
	default:
		return false, false
	}
}

func packageRunnerLooksVerifying(args []string, extraRunAliases ...string) bool {
	if len(args) == 0 {
		return false
	}
	if isVerificationScript(args[0]) {
		return true
	}
	if args[0] == "run" || args[0] == "exec" || stringIn(args[0], extraRunAliases...) {
		return len(args) > 1 && isVerificationScript(args[1])
	}
	return false
}

func wrappedCallVerificationTrust(call shellCall) (trusted, projectRule bool) {
	inner := unwrapShellCall(call)
	if inner.Name == "" {
		return false, false
	}
	return callVerificationTrust(inner)
}

func unwrapShellCall(call shellCall) shellCall {
	args := call.Args
	switch call.Name {
	case "time":
	case "timeout":
		args = stripTimeoutOptions(args)
		if len(args) > 0 {
			args = args[1:]
		}
	case "env":
		for len(args) > 0 && (strings.Contains(args[0], "=") || strings.HasPrefix(args[0], "-")) {
			opt := args[0]
			if opt == "--" {
				args = args[1:]
				break
			}
			if envOptionTakesValue(opt) && len(args) > 1 {
				args = args[2:]
				continue
			}
			args = args[1:]
		}
	}
	if len(args) == 0 {
		return shellCall{}
	}
	return shellCall{Name: args[0], Args: args[1:]}
}

func stripTimeoutOptions(args []string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		opt := args[0]
		args = args[1:]
		if timeoutOptionTakesValue(opt) && len(args) > 0 {
			args = args[1:]
		}
	}
	return args
}

func timeoutOptionTakesValue(opt string) bool {
	switch opt {
	case "-k", "--kill-after", "-s", "--signal":
		return true
	default:
		return false
	}
}

func envOptionTakesValue(opt string) bool {
	switch opt {
	case "-u", "--unset", "-c", "--chdir", "-s", "--split-string":
		return true
	default:
		return false
	}
}

func callLooksMutating(call shellCall) bool {
	switch call.Name {
	case "mkdir", "touch", "cp", "mv", "rm", "rmdir", "chmod", "chown":
		return true
	case "tee":
		return hasNonOptionArg(call.Args)
	case "sed":
		return hasAnyArgPrefix(call.Args, "-i")
	case "perl":
		return perlLooksInPlace(call.Args)
	case "git":
		return hasFirstArg(call, "add", "commit", "reset", "checkout", "merge", "rebase")
	case "npm":
		return hasFirstArg(call, "install", "i", "ci")
	case "pnpm":
		return hasFirstArg(call, "install", "i")
	case "yarn":
		return len(call.Args) == 0 || hasFirstArg(call, "install", "add")
	case "go":
		return hasFirstArg(call, "install", "get")
	case "cargo":
		return hasFirstArg(call, "install")
	case "make":
		return true
	default:
		return false
	}
}

func hasFirstArg(call shellCall, values ...string) bool {
	if len(call.Args) == 0 {
		return false
	}
	return stringIn(call.Args[0], values...)
}

func hasAnyArg(args []string, values ...string) bool {
	for _, arg := range args {
		if stringIn(arg, values...) {
			return true
		}
	}
	return false
}

func hasAnyArgPrefix(args []string, prefixes ...string) bool {
	for _, arg := range args {
		for _, prefix := range prefixes {
			if strings.HasPrefix(arg, prefix) {
				return true
			}
		}
	}
	return false
}

func perlLooksInPlace(args []string) bool {
	for _, arg := range args {
		if arg == "-i" || strings.HasPrefix(arg, "-i") {
			return true
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "i") {
			return true
		}
	}
	return false
}

func hasNonOptionArg(args []string) bool {
	for _, arg := range args {
		if arg != "" && !strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

func hasArgSequence(args []string, seq ...string) bool {
	if len(seq) == 0 || len(args) < len(seq) {
		return false
	}
	for i := 0; i <= len(args)-len(seq); i++ {
		ok := true
		for j := range seq {
			if args[i+j] != seq[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func isVerificationScript(s string) bool {
	for _, prefix := range []string{"test", "tests", "lint", "check", "verify", "typecheck"} {
		if s == prefix || strings.HasPrefix(s, prefix+":") || strings.HasPrefix(s, prefix+"-") {
			return true
		}
	}
	return stringIn(s, "build", "ci", "tsc", "vitest", "jest", "mocha", "eslint")
}

func stringIn(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

var fallbackVerificationPrefixes = []string{
	"go test", "go vet", "npm test", "npm run test", "npm run lint",
	"npm run typecheck", "npm run build", "pnpm test", "pnpm lint",
	"pnpm typecheck", "yarn test", "yarn lint", "yarn typecheck",
	"bun test", "cargo test", "cargo clippy", "cargo check", "pytest",
	"python -m pytest", "make test", "make lint", "make check",
	"make verify", "just test", "just check", "task test", "task check",
}

var fallbackVerificationProjectRules = map[string]bool{
	"npm test": true, "npm run test": true, "npm run lint": true,
	"npm run typecheck": true, "npm run build": true,
	"pnpm test": true, "pnpm lint": true, "pnpm typecheck": true,
	"yarn test": true, "yarn lint": true, "yarn typecheck": true,
	"bun test": true, "make test": true, "make lint": true,
	"make check": true, "make verify": true, "just test": true,
	"just check": true, "task test": true, "task check": true,
}

var fallbackMutatingPrefixes = []string{
	"mkdir", "touch ", "cp ", "mv ", "rm ", "rmdir", "chmod ", "chown ",
	"git add", "git commit", "git reset", "npm install", "pnpm install",
	"yarn add", "go install", "cargo install", "make ",
}
