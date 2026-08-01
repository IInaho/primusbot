package index

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	indexcore "nekocode/bot/tools/builtin/index/core"
	"nekocode/bot/tools/runtime/core"
)

// IndexTool exposes the code graph to the agent via the tool system.
// It lazily initializes its own Manager on first use, so registration
// requires no external setup.
type IndexTool struct {
	mgrOnce sync.Once
	mgr     indexcore.Manager
	mgrErr  error
}

// NewIndexTool creates a new index tool. The Manager is built lazily on
// first Execute using the process working directory.
func NewIndexTool() *IndexTool {
	return &IndexTool{}
}

func (t *IndexTool) ensureInit() (indexcore.Manager, error) {
	t.mgrOnce.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			t.mgrErr = fmt.Errorf("resolve working directory: %w", err)
			return
		}
		t.mgr, t.mgrErr = indexcore.NewManager(cwd)
		if t.mgrErr != nil {
			return
		}
		t.mgrErr = t.mgr.Init()
	})
	return t.mgr, t.mgrErr
}

func (t *IndexTool) Name() string { return "index" }
func (t *IndexTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	return core.ModeParallel
}

func (t *IndexTool) Description() string {
	return "Query the project index for symbols (symbol:), files (file:), package dependencies (deps:), text (search:), or an architecture overview (skeleton). Use it when code structure is the question; confirm relevant source before editing because index results are navigation evidence, not current file contents."
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
	mgr, err := t.ensureInit()
	if err != nil {
		return fmt.Sprintf("Index initialization failed: %v", err), nil
	}

	query, _ := args["query"].(string)
	if query == "" {
		return "Missing required parameter 'query'. Usage: query=\"file:manager.go\" or query=\"symbol:Agent\". Note: 'file' is not a parameter name — use query=\"file:<name>\".", nil
	}

	if strings.HasPrefix(query, "search:") {
		value := strings.TrimSpace(strings.TrimPrefix(query, "search:"))
		return querySearch(mgr, value), nil
	}

	if query == "skeleton" {
		return mgr.Skeleton(), nil
	}

	prefix, value, ok := strings.Cut(query, ":")
	if !ok {
		return "Invalid query format. Use '<prefix>:<value>' (e.g. \"file:manager.go\", \"symbol:Agent\") or \"skeleton\".", nil
	}
	value = strings.TrimSpace(value)

	switch prefix {
	case "symbol":
		return querySymbol(mgr, value), nil
	case "deps":
		return queryDeps(mgr, value), nil
	case "file":
		return queryFile(mgr, value), nil
	default:
		return fmt.Sprintf("Unknown query prefix '%s'. Available: symbol, deps, file, search, skeleton", prefix), nil
	}
}

func querySymbol(mgr indexcore.Manager, name string) string {
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

func queryDeps(mgr indexcore.Manager, pkgPath string) string {
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

func queryFile(mgr indexcore.Manager, name string) string {
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

func querySearch(mgr indexcore.Manager, term string) string {
	nodes, err := mgr.Search(term, 50)
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
