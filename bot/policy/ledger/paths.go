package ledger

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"nekocode/bot/policy/internal/shellscan"
)

type pathSet map[string]struct{}

func extractReadPaths(name string, args map[string]any) []string {
	switch name {
	case "read":
		if p, _ := args["path"].(string); p != "" {
			return []string{filepath.Clean(p)}
		}
	case "bash", "shell":
		cmd, _ := args["command"].(string)
		return cleanPaths(extractBashReadPaths(cmd))
	}
	return nil
}

func extractModifiedPaths(ev ToolEvent) []string {
	if ev.Error != "" {
		return nil
	}
	switch ev.Name {
	case "write", "edit":
		if p, _ := ev.Args["path"].(string); p != "" {
			return []string{filepath.Clean(p)}
		}
	case "bash", "shell":
		cmd, _ := ev.Args["command"].(string)
		return cleanPaths(extractBashWritePaths(cmd))
	}
	return nil
}

func newPathSetFrom(paths []string) pathSet {
	s := make(pathSet, len(paths))
	s.addAll(paths)
	return s
}

func (s pathSet) add(path string) {
	if path != "" {
		s[filepath.Clean(path)] = struct{}{}
	}
}

func (s pathSet) addAll(paths []string) {
	for _, path := range paths {
		s.add(path)
	}
}

func (s pathSet) has(path string) bool {
	if path == "" {
		return false
	}
	_, ok := s[filepath.Clean(path)]
	return ok
}

func (s pathSet) sorted() []string {
	return slices.Sorted(maps.Keys(s))
}

func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			out = append(out, filepath.Clean(path))
		}
	}
	return out
}

func commandArg(args map[string]any) string {
	command, _ := args["command"].(string)
	return strings.TrimSpace(command)
}

func extractBashReadPaths(command string) []string {
	scan := shellscan.ScanShell(command)
	if scan.OK {
		var out []string
		for _, fields := range scan.Calls {
			out = append(out, bashReadPathsForFields(fields)...)
		}
		return out
	}
	return bashReadPathsForFields(shellscan.Fields(strings.TrimSpace(command)))
}

func bashReadPathsForFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	switch filepath.Base(fields[0]) {
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
	default:
		return nil
	}
}

func extractBashWritePaths(command string) []string {
	scan := shellscan.ScanShell(command)
	if scan.OK {
		var out []string
		for _, path := range scan.RedirectTargets {
			out = appendPathArg(out, path)
		}
		for _, fields := range scan.Calls {
			out = append(out, bashCommandWritePathsForFields(fields)...)
		}
		return out
	}
	fields := shellscan.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return nil
	}
	return bashCommandWritePathsForFields(fields)
}

func bashCommandWritePathsForFields(fields []string) []string {
	var out []string
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch {
		case isShellBoundary(field):
			continue
		case isRedirectOperator(field):
			if i+1 < len(fields) {
				out = appendPathArg(out, fields[i+1])
				i++
			}
		case redirectPath(field) != "":
			out = appendPathArg(out, redirectPath(field))
		case filepath.Base(field) == "tee":
			paths, next := teeWritePaths(fields, i+1)
			out = append(out, paths...)
			i = next
		case filepath.Base(field) == "sed":
			paths, next := sedWritePaths(fields, i+1)
			out = append(out, paths...)
			i = next
		case shellscan.IsMutatingCommand(filepath.Base(field)):
			paths, next := commandWritePaths(filepath.Base(field), fields, i+1)
			out = append(out, paths...)
			i = next
		}
	}
	return out
}

func isShellBoundary(value string) bool {
	switch value {
	case "|", "||", "&&", ";":
		return true
	default:
		return false
	}
}

func isRedirectOperator(value string) bool {
	switch value {
	case ">", ">>", ">|", "&>", "&>>", "1>", "1>>", "2>", "2>>":
		return true
	default:
		return false
	}
}

func redirectPath(value string) string {
	for _, prefix := range []string{"&>>", "&>", "1>>", "1>", "2>>", "2>", ">>", ">|", ">"} {
		if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
			return value[len(prefix):]
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
		if !strings.HasPrefix(fields[i], "-") {
			out = appendPathArg(out, fields[i])
		}
	}
	return out, i
}

func sedWritePaths(fields []string, start int) ([]string, int) {
	var out []string
	i := start
	inPlace := false
	scriptSeen := false
	for ; i < len(fields); i++ {
		field := fields[i]
		if isShellBoundary(field) {
			break
		}
		if field == "-i" || strings.HasPrefix(field, "-i") {
			inPlace = true
			continue
		}
		if strings.HasPrefix(field, "-") {
			continue
		}
		if !scriptSeen {
			scriptSeen = true
			continue
		}
		if inPlace {
			out = appendPathArg(out, field)
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
		if !strings.HasPrefix(fields[i], "-") {
			args = append(args, fields[i])
		}
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
	switch filepath.Clean(arg) {
	case "/dev/null", "/dev/stdout", "/dev/stderr":
		return false
	default:
		return true
	}
}

func nonOptionArgs(args []string) []string {
	var out []string
	for _, arg := range args {
		if arg != "--" && !strings.HasPrefix(arg, "-") && looksLikePathArg(arg) {
			out = append(out, arg)
		}
	}
	return out
}

func skipOptionValues(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" || arg == "-c" || arg == "--lines" || arg == "--bytes" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-n") || strings.HasPrefix(arg, "-c") ||
			strings.HasPrefix(arg, "--lines=") || strings.HasPrefix(arg, "--bytes=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func leadingPathArgs(args []string) []string {
	var out []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !looksLikePathArg(arg) {
			break
		}
		out = append(out, arg)
	}
	return out
}

func grepPathArgs(args []string) []string {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if consumesNextArg(arg) {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
		}
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
		for i, arg := range args[1:] {
			if arg == "--" {
				return filterPathArgs(args[i+2:])
			}
		}
	}
	return nil
}

func consumesNextArg(arg string) bool {
	switch arg {
	case "-e", "-f", "-g", "-m", "-A", "-B", "-C", "--regexp", "--file",
		"--glob", "--max-count", "--after-context", "--before-context", "--context":
		return true
	default:
		return false
	}
}

func filterPathArgs(args []string) []string {
	var out []string
	for _, arg := range args {
		if looksLikePathArg(arg) {
			out = append(out, arg)
		}
	}
	return out
}

func looksLikePathArg(arg string) bool {
	if arg == "" || strings.HasPrefix(arg, "-") {
		return false
	}
	return strings.Contains(arg, "/") || strings.Contains(arg, ".") || arg == "." || arg == ".."
}
