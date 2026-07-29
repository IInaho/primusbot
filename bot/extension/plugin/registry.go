package plugin

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"nekocode/util/fs"
)

// registry manages plugin lifecycle.
type registry struct {
	mu       sync.RWMutex
	plugins  map[string]*Plugin
	baseDirs []string

	Logf func(string, ...any)
}

// DefaultDirs returns plugin search paths (project > user).
func defaultDirs() []string {
	return fs.NekocodeDirs("plugins")
}

// newRegistry creates a plugin registry scanning baseDirs.
func newRegistry(baseDirs []string) *registry {
	return &registry{
		plugins:  make(map[string]*Plugin),
		baseDirs: baseDirs,
	}
}

// LoadAll scans all base dirs and loads plugin manifests.
func (r *registry) LoadAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	regData := r.loadRegistryFile()
	seen := make(map[string]bool)
	for _, baseDir := range r.baseDirs {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginDir := filepath.Join(baseDir, entry.Name())
			if !hasManifest(pluginDir) {
				continue
			}
			r.loadPlugin(pluginDir, regData, seen)
		}
	}
}

func (r *registry) loadPlugin(pluginDir string, regData registryJSON, seen map[string]bool) *Plugin {
	m, err := parseManifest(pluginDir)
	if err != nil {
		if r.Logf != nil {
			r.Logf("plugin: skip %s: %v", pluginDir, err)
		}
		return nil
	}
	if seen[m.Name] {
		return nil
	}
	seen[m.Name] = true

	enabled := true
	source := ""
	var installedAt time.Time
	if re, ok := regData.Plugins[m.Name]; ok {
		enabled = re.Enabled
		source = re.Source
		if t, err := time.Parse(time.RFC3339, re.InstalledAt); err == nil {
			installedAt = t
		}
	}

	p := newPluginFromManifest(m, pluginDir, source)
	p.Enabled = enabled
	p.InstalledAt = installedAt
	r.plugins[m.Name] = p
	return p
}

func newPluginFromManifest(m *Manifest, dir, source string) *Plugin {
	return &Plugin{
		Manifest:       *m,
		Dir:            dir,
		Source:         source,
		Enabled:        true,
		InstalledAt:    time.Now(),
		HasInstallStub: fileExists(filepath.Join(dir, "install.sh")),
	}
}

// Uninstall removes a plugin from disk and registry.
func (r *registry) Uninstall(name string) error {
	r.mu.RLock()
	p, ok := r.plugins[name]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	if err := os.RemoveAll(p.Dir); err != nil {
		return fmt.Errorf("remove plugin dir: %w", err)
	}

	r.mu.Lock()
	delete(r.plugins, name)
	r.saveRegistryFile()
	r.mu.Unlock()
	return nil
}

// List returns all installed plugins sorted by name.
func (r *registry) List() []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.SortedFunc(maps.Values(r.plugins), func(a, b *Plugin) int {
		return strings.Compare(a.Name, b.Name)
	})
}

// Get returns a plugin by name.
func (r *registry) Get(name string) (*Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// Enable enables a plugin.
func (r *registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Enabled = true
	r.saveRegistryFile()
	return nil
}

// Disable disables a plugin without removing it.
func (r *registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Enabled = false
	r.saveRegistryFile()
	return nil
}

// SkillDirs returns all skill directories from enabled plugins.
func (r *registry) SkillDirs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var dirs []string
	for _, p := range r.plugins {
		if p.Enabled {
			dirs = append(dirs, p.SkillDirs()...)
		}
	}
	return dirs
}
// registryJSON is the on-disk format for ~/.nekocode/plugins/registry.json.
type registryJSON struct {
	Plugins map[string]registryEntry `json:"plugins"`
}

type registryEntry struct {
	Version     string `json:"version"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
	InstalledAt string `json:"installedAt"`
}

func userPluginDir() (string, error) {
	return filepath.Join(fs.NekocodeHome(), "plugins"), nil
}

func (r *registry) registryPath() (string, error) {
	dir, err := userPluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "registry.json"), nil
}

func (r *registry) loadRegistryFile() registryJSON {
	path, err := r.registryPath()
	if err != nil {
		return registryJSON{Plugins: make(map[string]registryEntry)}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return registryJSON{Plugins: make(map[string]registryEntry)}
	}
	var reg registryJSON
	if err := json.Unmarshal(data, &reg); err != nil {
		return registryJSON{Plugins: make(map[string]registryEntry)}
	}
	if reg.Plugins == nil {
		reg.Plugins = make(map[string]registryEntry)
	}
	return reg
}

func (r *registry) saveRegistryFile() {
	path, err := r.registryPath()
	if err != nil {
		return
	}
	reg := registryJSON{Plugins: make(map[string]registryEntry)}
	for name, p := range r.plugins {
		reg.Plugins[name] = registryEntry{
			Version:     p.Version,
			Source:      p.Source,
			Enabled:     p.Enabled,
			InstalledAt: p.InstalledAt.Format(time.RFC3339),
		}
	}
	data, _ := json.MarshalIndent(reg, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, data, 0o644)
}
