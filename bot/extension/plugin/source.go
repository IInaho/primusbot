package plugin

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	utilhttp "nekocode/util/http"
	"nekocode/util/runtime"
	"nekocode/util/text"
)

func sourceToRawURL(source string) string {
	s := source
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if !strings.HasPrefix(s, "github.com/") && !strings.HasPrefix(s, "raw.githubusercontent.com/") {
		return ""
	}
	if strings.HasPrefix(s, "github.com/") {
		clean := strings.TrimSuffix(strings.TrimSuffix(s, ".git"), "/")
		parts := strings.SplitN(clean, "/", 6)
		if len(parts) < 3 {
			return ""
		}
		branch := "main"
		if len(parts) >= 5 && parts[3] == "tree" {
			branch = parts[4]
		}
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/.claude-plugin/plugin.json", parts[1], parts[2], branch)
	}
	if !strings.Contains(s, ".claude-plugin") {
		s = strings.TrimSuffix(s, "/") + "/.claude-plugin/plugin.json"
	}
	return "https://" + s
}

func isLocalPath(s string) bool {
	return strings.HasPrefix(s, "./") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") ||
		(!strings.Contains(s, "://") && !text.LooksLikeGit(s))
}

func expandPluginEnv(env map[string]string, pluginRoot string) map[string]string {
	if env == nil {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = expandPluginPath(v, pluginRoot)
	}
	return out
}

func expandPluginPath(s, pluginRoot string) string {
	s = strings.ReplaceAll(s, "${CLAUDE_PLUGIN_ROOT}", pluginRoot)
	return strings.ReplaceAll(s, "${PLUGIN_ROOT}", pluginRoot)
}

func ExpandPluginMCPConfig(cfg MCPServerConfig, pluginRoot string) MCPServerConfig {
	cfg.Command = expandPluginPath(cfg.Command, pluginRoot)
	for i := range cfg.Args {
		cfg.Args[i] = expandPluginPath(cfg.Args[i], pluginRoot)
	}
	cfg.Env = expandPluginEnv(cfg.Env, pluginRoot)
	return cfg
}

var fetchClient = &http.Client{
	Transport: utilhttp.SharedTransport,
	Timeout:   10 * time.Second,
}

func fetchURL(url string) ([]byte, error) {
	ctx, cancel := runtime.ShortContext()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64*1024))
}
