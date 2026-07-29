package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nekocode/util/text"
)

// PreviewFromPath creates a Plugin from a local path without installing.
func (r *registry) PreviewFromPath(source string) (*Plugin, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if !hasManifest(abs) {
		return nil, fmt.Errorf("no .claude-plugin/plugin.json found in %s", abs)
	}
	m, err := parseManifest(abs)
	if err != nil {
		return nil, err
	}
	return newPluginFromManifest(m, abs, source), nil
}

// Install clones or copies a plugin from source into user plugin dir.
func (r *registry) Install(source string) (*Plugin, error) {
	userDir, err := userPluginDir()
	if err != nil {
		return nil, fmt.Errorf("user plugin dir: %w", err)
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, fmt.Errorf("create plugin dir: %w", err)
	}

	pluginDir, err := r.installToUserDir(userDir, source)
	if err != nil {
		return nil, err
	}

	m, err := parseManifest(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("parse installed manifest: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	p := newPluginFromManifest(m, pluginDir, source)
	r.plugins[m.Name] = p
	r.saveRegistryFile()
	return p, nil
}

func (r *registry) installToUserDir(userDir, source string) (string, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") ||
		strings.Contains(source, "github.com") || strings.Contains(source, "gitlab.com") {
		pluginDir := filepath.Join(userDir, repoName(source))
		if err := r.gitClone(source, pluginDir); err != nil {
			return "", err
		}
		return pluginDir, nil
	}
	if text.LooksLikeGit(source) {
		url := "https://github.com/" + source
		pluginDir := filepath.Join(userDir, strings.ReplaceAll(source, "/", "-"))
		if err := r.gitClone(url, pluginDir); err != nil {
			return "", err
		}
		return pluginDir, nil
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !hasManifest(abs) {
		return "", fmt.Errorf("no .claude-plugin/plugin.json found in %s", abs)
	}
	m, err := parseManifest(abs)
	if err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}
	pluginDir := filepath.Join(userDir, m.Name)
	if err := copyDir(abs, pluginDir); err != nil {
		return "", fmt.Errorf("copy plugin: %w", err)
	}
	return pluginDir, nil
}

func (r *registry) gitClone(url, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return runGit(dest, "pull", "--ff-only")
	}
	return runGit("", "clone", "--depth", "1", url, dest)
}

func repoName(url string) string {
	s := strings.TrimSuffix(url, ".git")
	s = strings.TrimSuffix(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "-" + parts[len(parts)-1]
	}
	return s
}
const gitTimeout = 60 * time.Second

func runGit(dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Run()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})
}
