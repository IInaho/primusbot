// Package checkpoint stores per-turn file snapshots and restores them on rewind.
package checkpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nekocode/util/fs"
)

const (
	manifestName       = "manifest.json"
	MaxTurnsPerSession = 10
)

type entry struct {
	Path     string `json:"path"`
	Before   string `json:"before"`           // absent | file
	Change   string `json:"change,omitempty"` // created | modified | deleted
	Snapshot string `json:"snapshot,omitempty"`
	Mode     uint32 `json:"mode,omitempty"`
}

type manifest struct {
	Turn      string    `json:"turn"`
	CreatedAt time.Time `json:"created_at"`
	Entries   []entry   `json:"entries"`
}

type Result struct {
	Turn  string
	Files int
	Paths []string
}

type FileChange struct {
	Path string
	Kind string
}

type TurnInfo struct {
	Turn      string
	CreatedAt time.Time
	Changes   []FileChange
}

type Manager struct {
	mu sync.Mutex

	root          string
	turns         map[string][]string
	next          map[string]int
	activeSession string
	active        manifest
	inFlight      map[string]int
}

func defaultRoot() string { return fs.NekocodeDataDir("checkpoints") }

func New(root string) *Manager {
	if root == "" {
		root = defaultRoot()
	}
	return &Manager{root: root, turns: make(map[string][]string), next: make(map[string]int)}
}

// Activate installs the persisted turn index for a session. It does not begin
// a new turn; Begin is called only when the agent handles user input.
func (m *Manager) Activate(session string, turns []string, next int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeSession = ""
	m.active = manifest{}
	m.inFlight = nil
	if !validID(session) {
		return
	}
	m.turns[session] = cleanTurns(turns)
	if highest := highestTurn(m.turns[session]); next < highest {
		next = highest
	}
	m.next[session] = next
}

func (m *Manager) Begin(session string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !validID(session) {
		return "", fmt.Errorf("checkpoint: invalid session %q", session)
	}
	m.next[session]++
	turn := strconv.Itoa(m.next[session])
	dir := m.turnDir(session, turn)
	if err := secureDir(m.root); err != nil {
		m.next[session]--
		return "", fmt.Errorf("checkpoint: secure root: %w", err)
	}
	if err := secureDir(filepath.Join(m.root, session)); err != nil {
		m.next[session]--
		return "", fmt.Errorf("checkpoint: secure session: %w", err)
	}
	if err := secureDir(dir); err != nil {
		m.next[session]--
		return "", fmt.Errorf("checkpoint: create turn: %w", err)
	}
	next := manifest{Turn: turn, CreatedAt: time.Now()}
	if err := writeManifest(dir, next); err != nil {
		m.next[session]--
		return "", fmt.Errorf("checkpoint: write turn: %w", err)
	}
	m.activeSession = session
	m.active = next
	m.inFlight = make(map[string]int)
	m.turns[session] = append(m.turns[session], turn)
	return turn, nil
}

// Finish closes the active turn, drops empty turns, and retains the latest ten
// complete turns. A turn is never split based on how many files it changed.
func (m *Manager) Finish(session string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeSession == "" || m.active.Turn == "" {
		return nil
	}
	if m.activeSession != session {
		return fmt.Errorf("checkpoint: active session is %q, not %q", m.activeSession, session)
	}
	if len(m.inFlight) > 0 {
		return fmt.Errorf("checkpoint: cannot finish while file tools are running")
	}
	defer func() {
		m.activeSession = ""
		m.active = manifest{}
		m.inFlight = nil
	}()

	turns := m.turns[session]
	if len(m.active.Entries) == 0 {
		if err := os.RemoveAll(m.activeDir()); err != nil {
			return fmt.Errorf("checkpoint: remove empty turn %s: %w", m.active.Turn, err)
		}
		m.turns[session] = removeTurn(turns, m.active.Turn)
		return nil
	}
	for len(turns) > MaxTurnsPerSession {
		oldest := turns[0]
		if err := os.RemoveAll(m.turnDir(session, oldest)); err != nil {
			return fmt.Errorf("checkpoint: prune turn %s: %w", oldest, err)
		}
		turns = turns[1:]
	}
	m.turns[session] = append([]string(nil), turns...)
	return nil
}

// Capture records path's state once for the active turn. A capture failure is
// returned to the executor so a write cannot proceed without a rewind point.
func (m *Manager) Capture(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeSession == "" || m.active.Turn == "" {
		return nil
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("checkpoint: path must be absolute: %s", path)
	}
	for _, entry := range m.active.Entries {
		if entry.Path == path {
			m.inFlight[path]++
			return nil
		}
	}

	entry := entry{Path: path, Before: "absent"}
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		entry.Change = "created"
	case err != nil:
		return fmt.Errorf("checkpoint: inspect %s: %w", path, err)
	case !info.Mode().IsRegular():
		return fmt.Errorf("checkpoint: unsupported non-file target %s", path)
	default:
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("checkpoint: read %s: %w", path, readErr)
		}
		entry.Before = "file"
		entry.Change = "modified"
		entry.Mode = uint32(info.Mode().Perm())
		entry.Snapshot = snapshotName(path)
		if err := writePrivateFile(filepath.Join(m.activeDir(), entry.Snapshot), data); err != nil {
			return fmt.Errorf("checkpoint: snapshot %s: %w", path, err)
		}
	}

	m.active.Entries = append(m.active.Entries, entry)
	if err := writeManifest(m.activeDir(), m.active); err != nil {
		m.active.Entries = m.active.Entries[:len(m.active.Entries)-1]
		if entry.Snapshot != "" {
			_ = os.Remove(filepath.Join(m.activeDir(), entry.Snapshot))
		}
		return fmt.Errorf("checkpoint: update manifest: %w", err)
	}
	m.inFlight[path]++
	return nil
}

// Finalize records the observed change kind and removes no-op captures.
func (m *Manager) Finalize(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeSession == "" || m.active.Turn == "" {
		return nil
	}
	path = filepath.Clean(path)
	if count := m.inFlight[path]; count > 1 {
		m.inFlight[path] = count - 1
		return nil
	} else {
		delete(m.inFlight, path)
	}
	for i := range m.active.Entries {
		entry := &m.active.Entries[i]
		if entry.Path != path {
			continue
		}
		changed, kind, err := m.changed(*entry)
		if err != nil {
			return err
		}
		if !changed {
			if entry.Snapshot != "" {
				_ = os.Remove(filepath.Join(m.activeDir(), entry.Snapshot))
			}
			m.active.Entries = append(m.active.Entries[:i], m.active.Entries[i+1:]...)
		} else {
			entry.Change = kind
		}
		return writeManifest(m.activeDir(), m.active)
	}
	return nil
}

func (m *Manager) Index(session string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.turns[session]...)
}

func (m *Manager) Next(session string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.next[session]
}

// History returns complete checkpoints newest first.
func (m *Manager) History(session string) ([]TurnInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	turns := m.turns[session]
	history := make([]TurnInfo, 0, len(turns))
	for i := len(turns) - 1; i >= 0; i-- {
		mf, err := readManifest(m.turnDir(session, turns[i]))
		if err != nil {
			return nil, err
		}
		if len(mf.Entries) == 0 {
			continue
		}
		info := TurnInfo{Turn: mf.Turn, CreatedAt: mf.CreatedAt, Changes: make([]FileChange, 0, len(mf.Entries))}
		for _, item := range mf.Entries {
			info.Changes = append(info.Changes, FileChange{Path: item.Path, Kind: item.Change})
		}
		history = append(history, info)
	}
	return history, nil
}

// Rewind restores the requested turn anchor and every newer turn. An empty
// target selects the latest turn. Rewound anchors are removed after success.
func (m *Manager) Rewind(session, target string) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	turns := m.turns[session]
	if len(turns) == 0 {
		return Result{}, fmt.Errorf("checkpoint: no turns available")
	}
	if len(m.inFlight) > 0 {
		return Result{}, fmt.Errorf("checkpoint: cannot rewind while file tools are running")
	}
	if target == "" {
		target = turns[len(turns)-1]
	}
	idx := indexOf(turns, target)
	if idx < 0 {
		return Result{}, fmt.Errorf("checkpoint: turn %q not found (available: %s)", target, strings.Join(turns, ", "))
	}

	result := Result{Turn: target}
	var restoreErr error
	for i := len(turns) - 1; i >= idx; i-- {
		turn := turns[i]
		mf, err := readManifest(m.turnDir(session, turn))
		if err != nil {
			restoreErr = errors.Join(restoreErr, err)
			continue
		}
		for j := len(mf.Entries) - 1; j >= 0; j-- {
			if err := m.restore(session, turn, mf.Entries[j]); err != nil {
				restoreErr = errors.Join(restoreErr, err)
				continue
			}
			result.Files++
			result.Paths = append(result.Paths, mf.Entries[j].Path)
		}
	}
	if restoreErr != nil {
		return result, restoreErr
	}
	for _, turn := range turns[idx:] {
		if err := os.RemoveAll(m.turnDir(session, turn)); err != nil {
			return result, fmt.Errorf("checkpoint: remove rewound turn %s: %w", turn, err)
		}
	}
	m.turns[session] = append([]string(nil), turns[:idx]...)
	m.activeSession = ""
	m.active = manifest{}
	m.inFlight = nil
	return result, nil
}

func (m *Manager) Delete(session string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !validID(session) {
		return fmt.Errorf("checkpoint: invalid session %q", session)
	}
	delete(m.turns, session)
	delete(m.next, session)
	if m.activeSession == session {
		m.activeSession = ""
		m.active = manifest{}
		m.inFlight = nil
	}
	return os.RemoveAll(filepath.Join(m.root, session))
}

func (m *Manager) changed(entry entry) (bool, string, error) {
	data, err := os.ReadFile(entry.Path)
	if entry.Before == "absent" {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		if err != nil {
			return false, "", fmt.Errorf("checkpoint: inspect result %s: %w", entry.Path, err)
		}
		return true, "created", nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, "deleted", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("checkpoint: inspect result %s: %w", entry.Path, err)
	}
	before, err := os.ReadFile(filepath.Join(m.activeDir(), entry.Snapshot))
	if err != nil {
		return false, "", fmt.Errorf("checkpoint: read snapshot %s: %w", entry.Path, err)
	}
	info, err := os.Stat(entry.Path)
	if err != nil {
		return false, "", fmt.Errorf("checkpoint: stat result %s: %w", entry.Path, err)
	}
	return !bytes.Equal(data, before) || uint32(info.Mode().Perm()) != entry.Mode, "modified", nil
}

func (m *Manager) restore(session, turn string, entry entry) error {
	switch entry.Before {
	case "absent":
		if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("checkpoint: remove created file %s: %w", entry.Path, err)
		}
		return nil
	case "file":
		data, err := os.ReadFile(filepath.Join(m.turnDir(session, turn), entry.Snapshot))
		if err != nil {
			return fmt.Errorf("checkpoint: read snapshot for %s: %w", entry.Path, err)
		}
		if err := os.MkdirAll(filepath.Dir(entry.Path), 0o755); err != nil {
			return fmt.Errorf("checkpoint: create parent for %s: %w", entry.Path, err)
		}
		tmp, err := os.CreateTemp(filepath.Dir(entry.Path), ".nekocode-rewind-*")
		if err != nil {
			return fmt.Errorf("checkpoint: create restore file for %s: %w", entry.Path, err)
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, err = tmp.Write(data); err == nil {
			err = tmp.Chmod(os.FileMode(entry.Mode))
		}
		if closeErr := tmp.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmpName, entry.Path)
		}
		if err != nil {
			return fmt.Errorf("checkpoint: restore %s: %w", entry.Path, err)
		}
		return nil
	default:
		return fmt.Errorf("checkpoint: invalid state %q for %s", entry.Before, entry.Path)
	}
}

func (m *Manager) activeDir() string { return m.turnDir(m.activeSession, m.active.Turn) }

func (m *Manager) turnDir(session, turn string) string {
	return filepath.Join(m.root, session, turn)
}

func snapshotName(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:16]) + ".bin"
}

func secureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func writePrivateFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func writeManifest(dir string, value manifest) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".manifest.tmp")
	if err := writePrivateFile(tmp, data); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, manifestName))
}

func readManifest(dir string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return manifest{}, fmt.Errorf("checkpoint: read manifest %s: %w", dir, err)
	}
	var value manifest
	if err := json.Unmarshal(data, &value); err != nil {
		return manifest{}, fmt.Errorf("checkpoint: decode manifest %s: %w", dir, err)
	}
	return value, nil
}

func validID(value string) bool {
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func cleanTurns(turns []string) []string {
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		if validID(turn) && indexOf(out, turn) < 0 {
			out = append(out, turn)
		}
	}
	return out
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func removeTurn(turns []string, target string) []string {
	idx := indexOf(turns, target)
	if idx < 0 {
		return turns
	}
	return append(turns[:idx:idx], turns[idx+1:]...)
}

func highestTurn(turns []string) int {
	highest := 0
	for _, turn := range turns {
		if value, err := strconv.Atoi(turn); err == nil && value > highest {
			highest = value
		}
	}
	return highest
}
