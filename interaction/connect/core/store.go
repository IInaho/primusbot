// Package core provides the shared building blocks for IM connectors
// (telegram, feishu, QQ, ...): the connect.json file layer, the pairing
// state machine, the connector run-state base, shared text commands, the
// stream throttle, and the event-dispatch loop.
//
// The package offers composable parts, not a base class: each channel keeps
// its own connector.go and wires these pieces together.
package core

import (
	"encoding/json"
	"os"
	"path/filepath"

	"nekocode/util/fs"
)

// FileStore reads and writes ~/.nekocode/connect.json one section at a
// time. Sections owned by other connectors are preserved verbatim as raw
// JSON, so concurrent connectors never clobber each other's configuration.
type FileStore struct {
	path string
}

// NewFileStore stores at an explicit path (tests).
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// DefaultFileStore stores at ~/.nekocode/connect.json.
func DefaultFileStore() *FileStore {
	return &FileStore{path: filepath.Join(fs.NekocodeHome(), "connect.json")}
}

// Path returns the underlying file path.
func (s *FileStore) Path() string { return s.path }

// Load unmarshals one section into out. A missing file or section leaves
// out untouched (zero value).
func (s *FileStore) Load(section string, out any) error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file map[string]json.RawMessage
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	raw, ok := file[section]
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// Save replaces one section, preserving every other section as-is, and
// writes the file atomically-ish with 0600 permissions.
func (s *FileStore) Save(section string, v any) error {
	file := map[string]json.RawMessage{}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &file)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	file[section] = raw
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fs.WriteFileWithDir(s.path, data, 0o600)
}
