package ledger

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"
	"nekocode/bot/policy/semantics"
)

type ToolEvent struct {
	Name      string
	Args      map[string]any
	Output    string
	Error     string
	Blocked   bool
	BlockText string
	Semantics semantics.Semantics
}

type Verification struct {
	Command     string
	Passed      bool
	Trusted     bool
	ProjectRule bool
	Output      string
}

type Ledger struct {
	mu sync.RWMutex

	readFiles      map[string]bool
	modifiedFiles  map[string]bool
	blockedTools   []ToolEvent
	toolErrors     []ToolEvent
	verifications  []Verification
	toolEventCount int
}

func New() *Ledger {
	return &Ledger{
		readFiles:     make(map[string]bool),
		modifiedFiles: make(map[string]bool),
	}
}

func (l *Ledger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.readFiles = make(map[string]bool)
	l.modifiedFiles = make(map[string]bool)
	l.blockedTools = nil
	l.toolErrors = nil
	l.verifications = nil
	l.toolEventCount = 0
}

func (l *Ledger) RecordTool(ev ToolEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.toolEventCount++
	if ev.Blocked {
		l.blockedTools = append(l.blockedTools, ev)
		return
	}
	if ev.Error != "" {
		l.toolErrors = append(l.toolErrors, ev)
	}
	if ev.Semantics.SourceProducing {
		for _, p := range extractReadPaths(ev.Name, ev.Args) {
			l.readFiles[p] = true
		}
	}
	if ev.Semantics.Mutating {
		for _, p := range extractModifiedPaths(ev) {
			l.modifiedFiles[p] = true
		}
	}
	if ev.Semantics.Verifying {
		l.verifications = append(l.verifications, Verification{
			Command:     commandArg(ev.Args),
			Passed:      ev.Error == "",
			Trusted:     ev.Semantics.VerificationTrusted,
			ProjectRule: ev.Semantics.VerificationProjectRule,
			Output:      ev.Output,
		})
	}
}

func (l *Ledger) Snapshot() Snapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s := Snapshot{
		ToolEventCount: len(l.blockedTools) + len(l.toolErrors),
	}
	for p := range l.readFiles {
		s.ReadFiles = append(s.ReadFiles, p)
	}
	for p := range l.modifiedFiles {
		s.ModifiedFiles = append(s.ModifiedFiles, p)
	}
	s.BlockedTools = append(s.BlockedTools, l.blockedTools...)
	s.ToolErrors = append(s.ToolErrors, l.toolErrors...)
	s.Verifications = append(s.Verifications, l.verifications...)
	s.ToolEventCount = l.toolEventCount
	return s
}

type Snapshot struct {
	ReadFiles      []string
	ModifiedFiles  []string
	BlockedTools   []ToolEvent
	ToolErrors     []ToolEvent
	Verifications  []Verification
	ToolEventCount int
}

// WasRead checks whether a specific file path has been read (tracked in ledger).
// The path is cleaned before comparison to match ledger storage format.
func (l *Ledger) WasRead(path string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	cleaned := filepath.Clean(path)
	return l.readFiles[cleaned]
}

func (s Snapshot) HasModifications() bool {
	return len(s.ModifiedFiles) > 0
}

func (s Snapshot) HasNonDocumentationModifications() bool {
	for _, path := range s.ModifiedFiles {
		if !isDocumentationPath(path) {
			return true
		}
	}
	return false
}

func (s Snapshot) HasPassingVerification() bool {
	for _, v := range s.Verifications {
		if v.Passed {
			return true
		}
	}
	return false
}

func (s Snapshot) Summary() string {
	return fmt.Sprintf("%d modified, %d verifications, %d tool errors, %d blocked tools",
		len(s.ModifiedFiles), len(s.Verifications), len(s.ToolErrors), len(s.BlockedTools))
}

func extractReadPaths(name string, args map[string]any) []string {
	switch name {
	case "read":
		if p, _ := args["path"].(string); p != "" {
			return []string{filepath.Clean(p)}
		}
	case "bash":
		cmd, _ := args["command"].(string)
		return cleanPaths(extractBashReadPaths(cmd))
	}
	return nil
}

func extractModifiedPaths(ev ToolEvent) []string {
	switch ev.Name {
	case "write", "edit":
		if ev.Error != "" {
			return nil
		}
		if p, _ := ev.Args["path"].(string); p != "" {
			return []string{filepath.Clean(p)}
		}
	case "bash":
		cmd, _ := ev.Args["command"].(string)
		return cleanPaths(extractBashWritePaths(cmd))
	}
	return nil
}

func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			out = append(out, filepath.Clean(p))
		}
	}
	return out
}

func commandArg(args map[string]any) string {
	cmd, _ := args["command"].(string)
	return strings.TrimSpace(cmd)
}

func extractBashReadPaths(cmd string) []string {
	if calls, ok := shellLiteralCalls(cmd); ok {
		var out []string
		for _, fields := range calls {
			out = append(out, bashReadPathsForFields(fields)...)
		}
		return out
	}
	fields := shellFields(strings.TrimSpace(cmd))
	return bashReadPathsForFields(fields)
}

func bashReadPathsForFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	name := filepath.Base(fields[0])
	switch name {
	case "cat", "less", "more", "file", "stat":
		return nonOptionArgs(fields[1:])
	case "head", "tail", "wc":
		return nonOptionArgs(skipOptionValues(fields[1:]))
	case "find", "fd":
		return leadingPathArgs(fields[1:])
	case "rg", "grep":
		return grepPathArgs(fields[1:])
	case "git":
		return gitPathArgs(fields[1:])
	}
	return nil
}

func extractBashWritePaths(cmd string) []string {
	if calls, ok := shellLiteralCalls(cmd); ok {
		out := extractBashRedirectWritePaths(cmd)
		for _, fields := range calls {
			out = append(out, bashCommandWritePathsForFields(fields)...)
		}
		return out
	}
	fields := shellFields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return nil
	}
	out := extractBashRedirectWritePaths(cmd)
	return append(out, bashCommandWritePathsForFields(fields)...)
}

func bashCommandWritePathsForFields(fields []string) []string {
	var out []string
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case isShellBoundary(f):
			continue
		case isRedirectOperator(f):
			if i+1 < len(fields) {
				out = appendPathArg(out, fields[i+1])
				i++
			}
		case redirectPath(f) != "":
			out = appendPathArg(out, redirectPath(f))
		case filepath.Base(f) == "tee":
			paths, next := teeWritePaths(fields, i+1)
			out = append(out, paths...)
			i = next
		case filepath.Base(f) == "sed":
			paths, next := sedWritePaths(fields, i+1)
			out = append(out, paths...)
			i = next
		case isPathMutatingCommand(filepath.Base(f)):
			paths, next := commandWritePaths(filepath.Base(f), fields, i+1)
			out = append(out, paths...)
			i = next
		}
	}
	return out
}

func shellLiteralCalls(cmd string) ([][]string, bool) {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return nil, false
	}
	var calls [][]string
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		var fields []string
		for i, arg := range call.Args {
			word := shellLiteralWord(arg)
			if i == 0 && word == "" {
				return true
			}
			fields = append(fields, word)
		}
		calls = append(calls, fields)
		return true
	})
	return calls, true
}

func isPathMutatingCommand(name string) bool {
	switch name {
	case "mkdir", "touch", "cp", "mv", "rm", "rmdir", "chmod", "chown":
		return true
	default:
		return false
	}
}

func extractBashRedirectWritePaths(cmd string) []string {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return nil
	}
	var out []string
	syntax.Walk(file, func(node syntax.Node) bool {
		redir, ok := node.(*syntax.Redirect)
		if !ok || !isWriteRedirect(redir.Op) {
			return true
		}
		if p := shellLiteralWord(redir.Word); p != "" {
			out = appendPathArg(out, p)
		}
		return true
	})
	return out
}

func isWriteRedirect(op syntax.RedirOperator) bool {
	return op == syntax.RdrOut || op == syntax.AppOut || op == syntax.ClbOut ||
		op == syntax.RdrAll || op == syntax.AppAll
}

func shellLiteralWord(w *syntax.Word) string {
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

func isShellBoundary(s string) bool {
	switch s {
	case "|", "||", "&&", ";":
		return true
	default:
		return false
	}
}

func isRedirectOperator(s string) bool {
	switch s {
	case ">", ">>", ">|", "&>", "&>>", "1>", "1>>", "2>", "2>>":
		return true
	default:
		return false
	}
}

func redirectPath(s string) string {
	for _, prefix := range []string{"&>>", "&>", "1>>", "1>", "2>>", "2>", ">>", ">|", ">"} {
		if strings.HasPrefix(s, prefix) && len(s) > len(prefix) {
			return s[len(prefix):]
		}
	}
	return ""
}

func teeWritePaths(fields []string, start int) ([]string, int) {
	var out []string
	i := start
	for ; i < len(fields); i++ {
		if isShellBoundary(fields[i]) {
			break
		}
		if strings.HasPrefix(fields[i], "-") {
			continue
		}
		out = appendPathArg(out, fields[i])
	}
	return out, i
}

func sedWritePaths(fields []string, start int) ([]string, int) {
	var out []string
	i := start
	inPlace := false
	scriptSeen := false
	for ; i < len(fields); i++ {
		f := fields[i]
		if isShellBoundary(f) {
			break
		}
		if f == "-i" || strings.HasPrefix(f, "-i") {
			inPlace = true
			continue
		}
		if strings.HasPrefix(f, "-") {
			continue
		}
		if !scriptSeen {
			scriptSeen = true
			continue
		}
		if inPlace {
			out = appendPathArg(out, f)
		}
	}
	return out, i
}

func commandWritePaths(name string, fields []string, start int) ([]string, int) {
	var args []string
	i := start
	for ; i < len(fields); i++ {
		if isShellBoundary(fields[i]) {
			break
		}
		if strings.HasPrefix(fields[i], "-") {
			continue
		}
		args = append(args, fields[i])
	}
	switch name {
	case "cp":
		if len(args) == 0 {
			return nil, i
		}
		return appendPathArg(nil, args[len(args)-1]), i
	case "chmod", "chown":
		if len(args) <= 1 {
			return nil, i
		}
		var out []string
		for _, arg := range args[1:] {
			out = appendPathArg(out, arg)
		}
		return out, i
	default:
		var out []string
		for _, arg := range args {
			out = appendPathArg(out, arg)
		}
		return out, i
	}
}

func appendPathArg(out []string, arg string) []string {
	if arg != "" && !strings.HasPrefix(arg, "-") && shouldTrackWritePath(arg) {
		return append(out, arg)
	}
	return out
}

func shouldTrackWritePath(arg string) bool {
	cleaned := filepath.Clean(arg)
	if cleaned == "/dev/null" || cleaned == "/dev/stdout" || cleaned == "/dev/stderr" {
		return false
	}
	return true
}

func shellFields(s string) []string {
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

func nonOptionArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if a == "--" {
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		if looksLikePathArg(a) {
			out = append(out, a)
		}
	}
	return out
}

func skipOptionValues(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-n" || a == "-c" || a == "--lines" || a == "--bytes" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-n") || strings.HasPrefix(a, "-c") ||
			strings.HasPrefix(a, "--lines=") || strings.HasPrefix(a, "--bytes=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func leadingPathArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if !looksLikePathArg(a) {
			break
		}
		out = append(out, a)
	}
	return out
}

func grepPathArgs(args []string) []string {
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if consumesNextArg(a) {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) <= 1 {
		return nil
	}
	return filterPathArgs(positional[1:])
}

func gitPathArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "diff", "show", "blame", "log":
		for i, a := range args[1:] {
			if a == "--" {
				return filterPathArgs(args[i+2:])
			}
		}
	}
	return nil
}

func consumesNextArg(a string) bool {
	switch a {
	case "-e", "-f", "-g", "-m", "-A", "-B", "-C", "--regexp", "--file",
		"--glob", "--max-count", "--after-context", "--before-context", "--context":
		return true
	}
	return false
}

func filterPathArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if looksLikePathArg(a) {
			out = append(out, a)
		}
	}
	return out
}

func looksLikePathArg(a string) bool {
	if a == "" || strings.HasPrefix(a, "-") {
		return false
	}
	return strings.Contains(a, "/") || strings.Contains(a, ".") || a == "." || a == ".."
}

func isDocumentationPath(path string) bool {
	cleaned := filepath.Clean(path)
	base := strings.ToLower(filepath.Base(cleaned))
	switch base {
	case "readme", "readme.md", "readme.mdx", "changelog", "changelog.md", "license", "license.md", "notice", "notice.md":
		return true
	}
	switch strings.ToLower(filepath.Ext(cleaned)) {
	case ".md", ".mdx", ".rst", ".adoc", ".txt":
		return true
	}
	return false
}
