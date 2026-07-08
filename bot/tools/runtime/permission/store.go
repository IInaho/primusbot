package permission

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"nekocode/bot/tools/runtime/core"
)

const permissionStoreVersion = 1

// Store persists capability grants and remembered rules for one project. The
// project's permissions live at <root>/.nekocode/permissions.json.
type Store struct {
	root string
}

// NewStore creates a store bound to a project root.
func NewStore(root string) *Store {
	return &Store{root: filepath.Clean(root)}
}

func (s *Store) projectPath() string {
	return filepath.Join(s.root, ".nekocode", "permissions.json")
}

type permissionFile struct {
	Version  int                          `json:"version"`
	Projects map[string]permissionProject `json:"projects"`
}

type permissionProject struct {
	Grants     []Grant         `json:"grants"`
	Rules      []Rule          `json:"rules,omitempty"`
	Workspaces []WorkspaceRoot `json:"workspaces,omitempty"`
}

// Grant records a user-authorized capability opening for a tool. The JSON is
// intentionally minimal: effect + tool + capabilities + the workspace it was
// granted for (for traceability) + the human-readable reason.
type Grant struct {
	Effect       string   `json:"effect"`
	Tool         string   `json:"tool"`
	Capabilities []string `json:"capabilities"`
	WritePaths   []string `json:"writePaths,omitempty"`
	Workspace    string   `json:"workspace,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

type WorkspaceRoot struct {
	Path   string `json:"path"`
	Access string `json:"access"`
}

func (s *Store) Match(tool string, req core.PermissionRequest) (Grant, bool) {
	tool = normalizeToolName(tool)
	if g, denied := s.Denied(tool, req); denied {
		return g, false
	}
	return s.find(tool, req, "allow", grantAllowsRequest)
}

func (s *Store) Denied(tool string, req core.PermissionRequest) (Grant, bool) {
	return s.find(normalizeToolName(tool), req, "deny", grantDeniesRequest)
}

func (s *Store) find(tool string, req core.PermissionRequest, effect string, match func(Grant, core.PermissionRequest) bool) (Grant, bool) {
	f, err := s.load()
	if err != nil {
		return Grant{}, false
	}
	project, ok := f.Projects[s.root]
	if !ok {
		return Grant{}, false
	}
	tool = normalizeToolName(tool)
	for _, g := range project.Grants {
		if g.Effect != effect || normalizeToolName(g.Tool) != tool {
			continue
		}
		if g.Workspace != "" && g.Workspace != s.root {
			continue
		}
		if match(g, req) {
			return g, true
		}
	}
	return Grant{}, false
}

func (s *Store) Allow(tool string, req core.PermissionRequest) error {
	if hasCapability(req.Capabilities, core.CapProcessHost) {
		return nil
	}
	tool = normalizeToolName(tool)
	f, err := s.load()
	if err != nil {
		return err
	}
	if f.Version == 0 {
		f.Version = permissionStoreVersion
	}
	if f.Projects == nil {
		f.Projects = map[string]permissionProject{}
	}
	p := f.Projects[s.root]
	g := Grant{
		Effect:       "allow",
		Tool:         tool,
		Capabilities: append([]string(nil), req.Capabilities...),
		WritePaths:   WritePathsFromRequest(req),
		Workspace:    s.root,
		Reason:       req.Reason,
	}
	for _, existing := range p.Grants {
		if existing.Effect == g.Effect && existing.Tool == g.Tool &&
			existing.Workspace == g.Workspace &&
			slices.Equal(existing.Capabilities, g.Capabilities) &&
			slices.Equal(normalizeWritePaths(existing.WritePaths), g.WritePaths) {
			return nil
		}
	}
	p.Grants = append(p.Grants, g)
	f.Projects[s.root] = p
	return s.save(f)
}

func (s *Store) RememberWorkspaceRoot(root WorkspaceRoot) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	if f.Version == 0 {
		f.Version = permissionStoreVersion
	}
	if f.Projects == nil {
		f.Projects = map[string]permissionProject{}
	}
	p := f.Projects[s.root]
	for i, existing := range p.Workspaces {
		if existing.Path != root.Path {
			continue
		}
		if root.Access == "read-write" || existing.Access != "read-write" {
			p.Workspaces[i] = root
		}
		f.Projects[s.root] = p
		return s.save(f)
	}
	p.Workspaces = append(p.Workspaces, root)
	f.Projects[s.root] = p
	return s.save(f)
}

func (s *Store) WorkspaceRoots() []WorkspaceRoot {
	f, err := s.load()
	if err != nil {
		return nil
	}
	p, ok := f.Projects[s.root]
	if !ok {
		return nil
	}
	return append([]WorkspaceRoot(nil), p.Workspaces...)
}

func (s *Store) load() (permissionFile, error) {
	path := s.projectPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return permissionFile{Version: permissionStoreVersion, Projects: map[string]permissionProject{}}, nil
	}
	if err != nil {
		return permissionFile{}, err
	}
	var f permissionFile
	if err := json.Unmarshal(data, &f); err != nil {
		return permissionFile{}, err
	}
	if f.Projects == nil {
		f.Projects = map[string]permissionProject{}
	}
	return f, nil
}

func (s *Store) save(f permissionFile) error {
	path := s.projectPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
