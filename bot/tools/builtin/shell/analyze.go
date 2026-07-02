package shell

import (
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type commandPlan struct {
	CommandClass string
	Workspace    string
	NeedsNetwork bool
	Unknown      bool
	Unsafe       bool
	WritePaths   []string
	CachePaths   []string
}

func analyzeCommand(cmdStr, workspace string) commandPlan {
	plan := commandPlan{CommandClass: "read-only", Workspace: workspace}
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmdStr), "")
	if err != nil {
		plan.CommandClass = "unknown"
		plan.Unknown = true
		return plan
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
			plan.CommandClass = "unknown"
			plan.Unknown = true
		case *syntax.CallExpr:
			classifyCall(n, &plan)
		case *syntax.Redirect:
			if n.Op == syntax.RdrOut || n.Op == syntax.AppOut || n.Op == syntax.RdrAll || n.Op == syntax.AppAll {
				if p := literalWord(n.Word); p != "" {
					plan.WritePaths = append(plan.WritePaths, normalizeCommandPath(workspace, p))
				}
			}
		}
		return true
	})
	plan.WritePaths = uniqueStrings(plan.WritePaths)
	plan.CachePaths = uniqueStrings(plan.CachePaths)
	return plan
}

func classifyCall(call *syntax.CallExpr, plan *commandPlan) {
	if len(call.Args) == 0 {
		return
	}
	name := literalWord(call.Args[0])
	switch name {
	case "":
		plan.CommandClass = "unknown"
		plan.Unknown = true
	case "eval", "source", ".":
		plan.CommandClass = "unknown"
		plan.Unsafe = true
	case "curl", "wget":
		plan.NeedsNetwork = true
		if plan.CommandClass == "read-only" {
			plan.CommandClass = "network"
		}
	case "npm", "pnpm", "yarn", "go", "cargo":
		classifyPackageLike(name, call, plan)
	case "rm", "rmdir":
		plan.CommandClass = "destructive"
		for _, arg := range call.Args[1:] {
			if p := literalWord(arg); p != "" && !strings.HasPrefix(p, "-") {
				plan.WritePaths = append(plan.WritePaths, normalizeCommandPath(plan.Workspace, p))
			}
		}
	case "mv", "cp", "mkdir", "touch", "chmod", "chown":
		if plan.CommandClass != "destructive" {
			plan.CommandClass = "fs-write"
		}
		for _, arg := range call.Args[1:] {
			if p := literalWord(arg); p != "" && !strings.HasPrefix(p, "-") {
				plan.WritePaths = append(plan.WritePaths, normalizeCommandPath(plan.Workspace, p))
			}
		}
	case "git":
		classifyGit(call, plan)
	}
}

func classifyPackageLike(name string, call *syntax.CallExpr, plan *commandPlan) {
	sub := ""
	if len(call.Args) > 1 {
		sub = literalWord(call.Args[1])
	}
	switch {
	case name == "npm" && (sub == "install" || sub == "i" || sub == "ci"):
		plan.CommandClass = "package-install"
		plan.NeedsNetwork = true
		plan.WritePaths = append(plan.WritePaths, filepath.Join(plan.Workspace, "node_modules"))
		plan.CachePaths = append(plan.CachePaths, userCachePath(".npm"))
	case name == "pnpm" && (sub == "install" || sub == "i"):
		plan.CommandClass = "package-install"
		plan.NeedsNetwork = true
		plan.WritePaths = append(plan.WritePaths, filepath.Join(plan.Workspace, "node_modules"))
		plan.CachePaths = append(plan.CachePaths, userCachePath(".pnpm-store"))
	case name == "yarn" && (sub == "" || sub == "install"):
		plan.CommandClass = "package-install"
		plan.NeedsNetwork = true
		plan.WritePaths = append(plan.WritePaths, filepath.Join(plan.Workspace, "node_modules"))
		plan.CachePaths = append(plan.CachePaths, userCachePath(".cache/yarn"))
	case name == "go" && (sub == "mod" || sub == "install" || sub == "get"):
		plan.CommandClass = "package-install"
		plan.NeedsNetwork = true
		plan.CachePaths = append(plan.CachePaths, userCachePath("go/pkg/mod"))
	case name == "cargo" && (sub == "fetch" || sub == "install" || sub == "build" || sub == "test"):
		if sub == "fetch" || sub == "install" {
			plan.CommandClass = "package-install"
			plan.NeedsNetwork = true
		} else if plan.CommandClass == "read-only" {
			plan.CommandClass = "test-build"
		}
		plan.CachePaths = append(plan.CachePaths, userCachePath(".cargo"))
	case plan.CommandClass == "read-only":
		plan.CommandClass = "test-build"
	}
}

func classifyGit(call *syntax.CallExpr, plan *commandPlan) {
	if len(call.Args) < 2 {
		return
	}
	switch literalWord(call.Args[1]) {
	case "add", "commit", "merge", "rebase", "checkout", "reset":
		plan.CommandClass = "vcs-write"
		plan.WritePaths = append(plan.WritePaths, plan.Workspace)
	case "push", "pull", "fetch", "clone":
		plan.NeedsNetwork = true
		if plan.CommandClass == "read-only" {
			plan.CommandClass = "network"
		}
	}
}

func literalWord(w *syntax.Word) string {
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

func normalizeCommandPath(workspace, p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~") {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(workspace, p)
}

func userCachePath(sub string) string {
	return "~/" + strings.TrimPrefix(sub, "/")
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
