package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/provider/types"
	"nekocode/util/fs"
)

type Snapshot struct {
	ID        string `json:"id"`
	CWD       string `json:"cwd"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`

	SystemPrompt    string          `json:"system_prompt"`
	Skills          string          `json:"skills"`
	Memory          string          `json:"memory"`
	Archive         string          `json:"archive"`
	Messages        []types.Message `json:"messages"`
	CompactBoundary int             `json:"compact_boundary"`

	ContextWindow     int `json:"context_window"`
	PromptTokens      int `json:"prompt_tokens"`
	CompletionTokens  int `json:"completion_tokens"`
	TrackerPrompt     int `json:"tracker_prompt_tokens,omitempty"`
	TrackerCompletion int `json:"tracker_completion_tokens,omitempty"`
	TrackerNewTokens  int `json:"tracker_new_tokens,omitempty"`
	CacheHitTokens    int `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens   int `json:"cache_miss_tokens,omitempty"`
	SubCount          int `json:"sub_count,omitempty"`
	SubTokens         int `json:"sub_tokens,omitempty"`
	SubCacheHit       int `json:"sub_cache_hit,omitempty"`
	SubCacheMiss      int `json:"sub_cache_miss,omitempty"`

	LoadedSkills []string        `json:"loaded_skills"`
	Ledger       ledger.Snapshot `json:"ledger"`
}

type Meta struct {
	ID        string `json:"id"`
	CWD       string `json:"cwd"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	MsgCount  int    `json:"msg_count"`
}

func dir() string {
	return filepath.Join(fs.NekocodeHome(), "sessions")
}

func newSnapshot(cwd string) *Snapshot {
	now := time.Now()
	return &Snapshot{
		ID:        fmt.Sprintf("%s-%09d", now.UTC().Format("20060102T150405"), now.Nanosecond()),
		CWD:       cwd,
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}
}

func load(id string) (*Snapshot, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	path := filepath.Join(dir(), id, "session.json")
	snapshot, err := fs.ReadJSONFile[*Snapshot](path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	if snapshot == nil || snapshot.ID != id {
		found := ""
		if snapshot != nil {
			found = snapshot.ID
		}
		return nil, fmt.Errorf("session id mismatch: requested %q, file contains %q", id, found)
	}
	return snapshot, nil
}

// Delete removes a session directory and all its contents.
func deleteSnapshot(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dir(), id))
}

func validateID(id string) error {
	if strings.TrimSpace(id) == "" || id == "." || id == ".." ||
		filepath.Base(id) != id || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("invalid session id: %q", id)
	}
	return nil
}

func (s *Snapshot) save() error {
	if s == nil {
		return fmt.Errorf("cannot save nil session")
	}
	if err := validateID(s.ID); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().Unix()
	d := filepath.Join(dir(), s.ID)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return fs.WriteFileWithDir(filepath.Join(d, "session.json"), data, 0o600)
}

// sessionMeta is a lightweight struct for deserializing only metadata from
// session.json, avoiding the cost of unmarshaling the full Messages array.
type sessionMeta struct {
	ID        string     `json:"id"`
	CWD       string     `json:"cwd"`
	CreatedAt int64      `json:"created_at"`
	UpdatedAt int64      `json:"updated_at"`
	Messages  []struct{} `json:"messages"` // only need len, not content
}

func loadMeta(id string) (Meta, error) {
	var sm sessionMeta
	path := filepath.Join(dir(), id, "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, err
	}
	_ = os.Chmod(path, 0o600)
	if err := json.Unmarshal(data, &sm); err != nil {
		return Meta{}, err
	}
	return Meta{
		ID: sm.ID, CWD: sm.CWD,
		CreatedAt: sm.CreatedAt, UpdatedAt: sm.UpdatedAt,
		MsgCount: len(sm.Messages),
	}, nil
}

func list() []Meta {
	entries, err := os.ReadDir(dir())
	if err != nil {
		return nil
	}
	var out []Meta
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		m, err := loadMeta(e.Name())
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

func (m Meta) Age() string {
	d := time.Since(time.Unix(m.UpdatedAt, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// CaptureContext stores context, usage, and loaded-skill state in the session.
func (s *Snapshot) CaptureContext(snap ctxmgr.ManagerSnapshot, promptTokens, completionTokens int, loaded map[string]bool) {
	if s == nil {
		return
	}
	s.SystemPrompt = snap.SystemPrompt
	s.Skills = snap.Skills
	s.Memory = snap.Memory
	s.Archive = snap.Archive
	s.Messages = snap.Messages
	s.CompactBoundary = snap.CompactBoundary
	s.ContextWindow = snap.Budget
	s.PromptTokens = promptTokens
	s.CompletionTokens = completionTokens
	s.TrackerPrompt = snap.Tracker.LastPromptTokens
	s.TrackerCompletion = snap.Tracker.LastCompTokens
	s.TrackerNewTokens = snap.Tracker.NewMessageTokens
	s.CacheHitTokens = snap.Tracker.CacheHitTokens
	s.CacheMissTokens = snap.Tracker.CacheMissTokens
	s.SubCount = snap.Tracker.Sub.Count
	s.SubTokens = snap.Tracker.Sub.TotalTokens
	s.SubCacheHit = snap.Tracker.Sub.CacheHitTokens
	s.SubCacheMiss = snap.Tracker.Sub.CacheMissTokens
	s.LoadedSkills = loadedSkillNames(loaded)
}

// ContextSnapshot restores the context-manager state stored in the session.
func (s *Snapshot) ContextSnapshot() ctxmgr.ManagerSnapshot {
	if s == nil {
		return ctxmgr.ManagerSnapshot{}
	}
	return ctxmgr.ManagerSnapshot{
		SystemPrompt:    s.SystemPrompt,
		Skills:          s.Skills,
		Archive:         s.Archive,
		Memory:          s.Memory,
		CompactBoundary: s.CompactBoundary,
		Messages:        s.Messages,
		Budget:          s.ContextWindow,
		Tracker: token.State{
			LastPromptTokens: s.TrackerPrompt,
			LastCompTokens:   s.TrackerCompletion,
			NewMessageTokens: s.TrackerNewTokens,
			CacheHitTokens:   s.CacheHitTokens,
			CacheMissTokens:  s.CacheMissTokens,
			Sub: token.SubStats{
				Count:           s.SubCount,
				TotalTokens:     s.SubTokens,
				CacheHitTokens:  s.SubCacheHit,
				CacheMissTokens: s.SubCacheMiss,
			},
		},
	}
}

func loadedSkillNames(loaded map[string]bool) []string {
	names := make([]string, 0, len(loaded))
	for name, ok := range loaded {
		if ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
