package index

import (
	graphpkg "nekocode/bot/index/internal/graph"
	"nekocode/bot/index/internal/projectctx"
	"nekocode/bot/index/internal/service"
)

// Target is the minimal context sink needed to apply project context.
type Target interface {
	Add(role, content string, source ...string)
	SetContextWindow(budget int)
}

// Manager is the public code-index contract. Callers outside this package
// should depend on this interface instead of internal graph/indexer packages.
type Manager interface {
	Init() error
	Close() error
	Rebuild() error
	Skeleton() string
	QuerySymbol(name string) []Symbol
	QueryDeps(pkgPath string) []string
	QueryFile(name string) []File
	Search(term string, limit int) ([]SearchResult, error)
}

type Symbol struct {
	Name    string
	Kind    string
	File    string
	Line    int
	PkgPath string
}

type File struct {
	Path string
}

type SearchResult struct {
	Name string
	Kind string
	File string
	Line int
}

type ApplyOptions struct {
	CWD             string
	ContextWindow   int
	LoadProjectText func(string) string
	NewManager      func(string) (Manager, error)
}

type Result struct {
	ProjectContext string
	IndexManager   Manager
}

type manager struct {
	inner *service.Manager
}

func NewManager(cwd string) (Manager, error) {
	mgr, err := service.NewManager(cwd)
	if err != nil {
		return nil, err
	}
	return &manager{inner: mgr}, nil
}

func (m *manager) Init() error {
	return m.inner.Init()
}

func (m *manager) Close() error {
	return m.inner.Close()
}

func (m *manager) Rebuild() error {
	return m.inner.Rebuild()
}

func (m *manager) Skeleton() string {
	return m.inner.Query(func(graph *graphpkg.Graph) string {
		return graph.FormatSkeleton(m.inner.CWD())
	})
}

func (m *manager) QuerySymbol(name string) []Symbol {
	graph := m.inner.Graph()
	if graph == nil {
		return nil
	}
	symbols := graph.QuerySymbol(name)
	out := make([]Symbol, 0, len(symbols))
	for _, s := range symbols {
		out = append(out, Symbol{
			Name:    s.Name,
			Kind:    s.Kind,
			File:    s.File,
			Line:    s.Line,
			PkgPath: s.PkgPath,
		})
	}
	return out
}

func (m *manager) QueryDeps(pkgPath string) []string {
	graph := m.inner.Graph()
	if graph == nil {
		return nil
	}
	return graph.QueryDeps(pkgPath)
}

func (m *manager) QueryFile(name string) []File {
	graph := m.inner.Graph()
	if graph == nil {
		return nil
	}
	files := graph.QueryFile(name)
	out := make([]File, 0, len(files))
	for _, f := range files {
		out = append(out, File{Path: f.Path})
	}
	return out
}

func (m *manager) Search(term string, limit int) ([]SearchResult, error) {
	if m.inner.Indexer() == nil {
		return nil, nil
	}
	nodes, err := m.inner.Indexer().SearchFTS(term, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, SearchResult{
			Name: n.Name,
			Kind: string(n.Kind),
			File: n.File,
			Line: n.Line,
		})
	}
	return out, nil
}

func Apply(target Target, opts ApplyOptions) Result {
	if target == nil {
		return Result{}
	}
	defer target.SetContextWindow(opts.ContextWindow)

	if opts.CWD == "" {
		return Result{}
	}

	loadProjectText := opts.LoadProjectText
	if loadProjectText == nil {
		loadProjectText = projectctx.LoadProjectContext
	}
	projectText := loadProjectText(opts.CWD)
	if projectText != "" {
		target.Add("system", projectText, "hint")
	}

	newManager := opts.NewManager
	if newManager == nil {
		newManager = NewManager
	}
	mgr, err := newManager(opts.CWD)
	if err != nil {
		return Result{ProjectContext: projectText}
	}
	if err := mgr.Init(); err != nil {
		return Result{ProjectContext: projectText}
	}
	if skeleton := mgr.Skeleton(); skeleton != "" {
		target.Add("system", skeleton, "hint")
	}

	return Result{ProjectContext: projectText, IndexManager: mgr}
}
