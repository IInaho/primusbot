package indextool

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"nekocode/bot/index"
	"nekocode/bot/tools/runtime/core"
)

// IndexTool exposes the code graph to the agent via the tool system.
// It holds a reference to the Manager so it always accesses the current graph
// (even after Manager.Rebuild replaces it).
type IndexTool struct {
	mgr index.Manager
}

// NewIndexTool creates a new index tool.
func NewIndexTool(mgr index.Manager) *IndexTool {
	return &IndexTool{mgr: mgr}
}

func (t *IndexTool) Name() string { return "index" }
func (t *IndexTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	return core.ModeParallel
}

func (t *IndexTool) Description() string {
	return "Pre-built project index. ALWAYS use this FIRST for: finding symbols (symbol:), finding files (file:), checking dependencies (deps:), full-text search (search:), or getting project overview (skeleton). Faster and more accurate than grep/glob for code structure queries."
}

func (t *IndexTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{
			Name:        "query",
			Type:        "string",
			Description: "Format: symbol:<name>, deps:<pkg>, file:<name>, search:<term>, or skeleton",
			Required:    true,
		},
	}
}

func (t *IndexTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "Missing required parameter 'query'. Usage: query=\"file:manager.go\" or query=\"symbol:Agent\". Note: 'file' is not a parameter name — use query=\"file:<name>\".", nil
	}

	if strings.HasPrefix(query, "search:") {
		value := strings.TrimSpace(strings.TrimPrefix(query, "search:"))
		return t.querySearch(value), nil
	}

	if query == "skeleton" {
		return t.mgr.Skeleton(), nil
	}

	prefix, value, ok := strings.Cut(query, ":")
	if !ok {
		return "Invalid query format. Use '<prefix>:<value>' (e.g. \"file:manager.go\", \"symbol:Agent\") or \"skeleton\".", nil
	}
	value = strings.TrimSpace(value)

	switch prefix {
	case "symbol":
		return querySymbol(t.mgr, value), nil
	case "deps":
		return queryDeps(t.mgr, value), nil
	case "file":
		return queryFile(t.mgr, value), nil
	default:
		return fmt.Sprintf("Unknown query prefix '%s'. Available: symbol, deps, file, search, skeleton", prefix), nil
	}
}

func querySymbol(mgr index.Manager, name string) string {
	symbols := mgr.QuerySymbol(name)
	if len(symbols) == 0 {
		return fmt.Sprintf("No symbols matching '%s' found in project index. Try grep for a broader search.", name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d symbol(s) matching '%s':\n", len(symbols), name)
	for _, s := range symbols {
		fmt.Fprintf(&b, "  %s %s — %s:%d (%s)\n", s.Kind, s.Name, shortenPath(s.File), s.Line, s.PkgPath)
	}
	return b.String()
}

func queryDeps(mgr index.Manager, pkgPath string) string {
	deps := mgr.QueryDeps(pkgPath)
	if deps == nil {
		return fmt.Sprintf("Package '%s' not found in project index or has no internal dependencies.", pkgPath)
	}
	sort.Strings(deps)
	var b strings.Builder
	fmt.Fprintf(&b, "Dependencies of %s (%d):\n", pkgPath, len(deps))
	for _, d := range deps {
		fmt.Fprintf(&b, "  %s\n", d)
	}
	return b.String()
}

func queryFile(mgr index.Manager, name string) string {
	files := mgr.QueryFile(name)
	if len(files) == 0 {
		return fmt.Sprintf("No files matching '%s' found in project index. The file may have been deleted or renamed — try glob to search the filesystem directly.", name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d file(s) matching '%s':\n", len(files), name)
	for _, f := range files {
		fmt.Fprintf(&b, "  %s\n", shortenPath(f.Path))
	}
	return b.String()
}

func (t *IndexTool) querySearch(term string) string {
	nodes, err := t.mgr.Search(term, 50)
	if err != nil {
		return fmt.Sprintf("Search error: %v", err)
	}
	if nodes == nil {
		return "Full-text search is not available (database not initialized)."
	}
	if len(nodes) == 0 {
		return fmt.Sprintf("No results for '%s' in project index.", term)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d result(s) for '%s':\n", len(nodes), term)
	for _, n := range nodes {
		fmt.Fprintf(&b, "  %s %s — %s:%d\n", n.Kind, n.Name, shortenPath(n.File), n.Line)
	}
	return b.String()
}

var (
	homeDir     string
	homeDirOnce sync.Once
)

func shortenPath(path string) string {
	homeDirOnce.Do(func() {
		homeDir, _ = os.UserHomeDir()
	})
	if homeDir != "" && strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	parts := strings.Split(path, "/")
	if len(parts) > 3 {
		parts = parts[len(parts)-3:]
	}
	return strings.Join(parts, "/")
}
