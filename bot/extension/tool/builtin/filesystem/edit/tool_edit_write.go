package edit

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeEditFile stages the complete result beside the target and commits it
// with one rename. Failures before the rename leave the original untouched.
func writeEditFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".nekocode-edit-*")
	if err != nil {
		return fmt.Errorf("create staged file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set staged file mode: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("stage content: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync staged content: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close staged file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit staged file: %w", err)
	}
	return nil
}
