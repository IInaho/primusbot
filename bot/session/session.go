package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"nekocode/bot/provider/types"
	"nekocode/util/fs"
)

// Manager owns only the current session identity and persistence lifecycle.
// Cross-module state capture and restore belongs to the bot assembly layer.
type Manager struct {
	mu   sync.Mutex
	cwd  string
	sess *Snapshot
}

// DefaultExportPath is the default context-export destination under ~/.nekocode/exports.
var DefaultExportPath = filepath.Join(fs.NekocodeDataDir("exports"), "nekocode-context.json")

// New creates a manager with a fresh current session.
func New(cwd string) *Manager {
	return &Manager{cwd: cwd, sess: newSnapshot(cwd)}
}

func (m *Manager) Current() *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sess
}

func (m *Manager) CurrentID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sess == nil {
		return ""
	}
	return m.sess.ID
}

func (m *Manager) set(sess *Snapshot) {
	m.mu.Lock()
	m.sess = sess
	m.mu.Unlock()
}

// StartNew replaces the current session identity.
func (m *Manager) StartNew() *Snapshot {
	sess := newSnapshot(m.cwd)
	m.set(sess)
	return sess
}

// ClearCurrent drops the current session identity.
func (m *Manager) ClearCurrent() {
	m.set(nil)
}

// Save persists snapshot as the current session. A session with no conversation
// history is removed from disk instead of being written, so empty sessions
// never show up as invalid records in session lists.
func (m *Manager) Save(sess *Snapshot) error {
	if sess == nil {
		return fmt.Errorf("session: cannot save nil snapshot")
	}
	if len(sess.Messages) == 0 {
		if err := deleteSnapshot(sess.ID); err != nil {
			return err
		}
		m.clearCurrentIfID(sess.ID)
		return nil
	}
	if err := sess.save(); err != nil {
		return err
	}
	m.set(sess)
	return nil
}

func (m *Manager) clearCurrentIfID(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sess != nil && m.sess.ID == id {
		m.sess = nil
	}
}

// Load validates and reads a persisted session without changing the current
// session identity. Callers can finish fail-closed cleanup before activation.
func (m *Manager) Load(id string) (*Snapshot, error) {
	sess, err := load(id)
	if err != nil {
		return nil, fmt.Errorf("session: load: %w", err)
	}
	return sess, nil
}

// Activate replaces the current session with a previously loaded snapshot.
func (m *Manager) Activate(sess *Snapshot) error {
	if sess == nil || sess.ID == "" {
		return fmt.Errorf("session: cannot activate an empty snapshot")
	}
	m.set(sess)
	return nil
}

func (m *Manager) List() []Meta {
	return list()
}

func (m *Manager) Delete(id string) error {
	return deleteSnapshot(id)
}

func ExportMessages(msgs []types.Message, path string) (string, error) {
	if path == "" {
		path = DefaultExportPath
	}
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal context: %w", err)
	}
	if err := fs.WriteFileWithDir(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return path, nil
}
