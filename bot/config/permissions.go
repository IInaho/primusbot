package config

// PermissionsConfig holds declarative permission rules (claude-code style).
// Rules are strings of the form "Tool(specifier)" (or bare "Tool"); the
// permission engine parses them and evaluates deny → ask → allow.
//
// Example:
//
//	"permissions": {
//	  "allow": ["Bash(npm run *)", "Edit(/src/**)"],
//	  "ask":   ["Bash(git push *)"],
//	  "deny":  ["Bash(curl *)", "Read(./.env)"],
//	  "sandbox": {
//	    "Bash(npm install *)": {"sandbox_mode": "workspace-write", "network": true, "writable_roots": ["~/.npm"]},
//	    "Bash(git status *)": {"sandbox_mode": "read-only"}
//	  }
//	}
type PermissionsConfig struct {
	Allow   []string                 `json:"allow,omitempty"`
	Ask     []string                 `json:"ask,omitempty"`
	Deny    []string                 `json:"deny,omitempty"`
	Sandbox map[string]SandboxConfig `json:"sandbox,omitempty"`
}

type SandboxConfig struct {
	SandboxMode   string   `json:"sandbox_mode,omitempty"`
	Network       bool     `json:"network,omitempty"`
	WritableRoots []string `json:"writable_roots,omitempty"`
}
