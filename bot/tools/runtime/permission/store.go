package permission

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/common"
)

const permissionStoreVersion = 1

type Store struct {
	path string
}

type permissionFile struct {
	Version  int                          `json:"version"`
	Projects map[string]permissionProject `json:"projects"`
}

type permissionProject struct {
	Grants []Grant `json:"grants"`
	// Rules holds user-approved permission rules remembered from the "ask"
	// dialog (the new declarative rule model). They are evaluated alongside
	// builtin and config-declared rules.
	Rules []Rule `json:"rules,omitempty"`
}

type Grant struct {
	ID           string         `json:"id"`
	Effect       string         `json:"effect"`
	Tool         string         `json:"tool"`
	Capabilities []string       `json:"capabilities"`
	Workspace    string         `json:"workspace"`
	Scope        string         `json:"scope"`
	Details      map[string]any `json:"details,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	ExpiresAt    *time.Time     `json:"expiresAt,omitempty"`
	Reason       string         `json:"reason,omitempty"`
}

func DefaultStore() *Store {
	return &Store{path: filepath.Join(common.NekocodeHome(), "permissions.json")}
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Match(tool string, req core.PermissionRequest) (Grant, bool) {
	if g, denied := s.Denied(tool, req); denied {
		return g, false
	}
	return s.find(tool, req, "allow", containsAllCapabilities)
}

func (s *Store) Denied(tool string, req core.PermissionRequest) (Grant, bool) {
	return s.find(tool, req, "deny", intersectsCapability)
}

func (s *Store) find(tool string, req core.PermissionRequest, effect string, capabilityMatch func([]string, []string) bool) (Grant, bool) {
	f, err := s.load()
	if err != nil {
		return Grant{}, false
	}
	workspace := workspaceFromPermission(req)
	project, ok := f.Projects[workspace]
	if !ok {
		return Grant{}, false
	}
	now := time.Now()
	for _, g := range project.Grants {
		if g.ExpiresAt != nil && now.After(*g.ExpiresAt) {
			continue
		}
		if g.Effect != effect || g.Tool != tool {
			continue
		}
		if g.Workspace != "" && workspace != "" && g.Workspace != workspace {
			continue
		}
		if capabilityMatch(g.Capabilities, req.Capabilities) {
			return g, true
		}
	}
	return Grant{}, false
}

func (s *Store) AllowProject(tool string, req core.PermissionRequest) error {
	if hasCapability(req.Capabilities, core.CapProcessHost) {
		return nil
	}
	workspace := workspaceFromPermission(req)
	if workspace == "" {
		return nil
	}
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
	p := f.Projects[workspace]
	g := Grant{
		ID:           "grant_" + time.Now().UTC().Format("20060102T150405.000000000"),
		Effect:       "allow",
		Tool:         tool,
		Capabilities: append([]string(nil), req.Capabilities...),
		Workspace:    workspace,
		Scope:        "project",
		Details:      req.Details,
		CreatedAt:    time.Now().UTC(),
		Reason:       req.Reason,
	}
	for _, existing := range p.Grants {
		if existing.Effect == g.Effect && existing.Tool == g.Tool && existing.Workspace == g.Workspace &&
			slices.Equal(existing.Capabilities, g.Capabilities) {
			return nil
		}
	}
	p.Grants = append(p.Grants, g)
	f.Projects[workspace] = p
	return s.save(f)
}

func (s *Store) load() (permissionFile, error) {
	data, err := os.ReadFile(s.path)
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
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(s.path, data, 0o600)
}

func workspaceFromPermission(req core.PermissionRequest) string {
	if req.Details == nil {
		return ""
	}
	v, _ := req.Details["workspace"].(string)
	return filepath.Clean(v)
}

func containsAllCapabilities(have, need []string) bool {
	for _, n := range need {
		if !hasCapability(have, n) {
			return false
		}
	}
	return true
}

func intersectsCapability(left, right []string) bool {
	for _, l := range left {
		if hasCapability(right, l) {
			return true
		}
	}
	return false
}

func hasCapability(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
