package edit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"nekocode/bot/tools/builtin/diff"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/toolutil"
	"nekocode/util/fs"
)

const maxUndoSnapshots = 128

type preflightResult struct {
	safePath         string
	normalizedBefore string
	lineEnding       string
}

func snapshotUndoPath(safePath string) string {
	h := sha256.Sum256([]byte(safePath))
	hash := hex.EncodeToString(h[:])[:16]
	return filepath.Join(fs.NekocodeDataDir("snapshots"), hash+"_"+filepath.Base(safePath)+".pre-edit")
}

func (t *EditTool) revertSnapshot(path string) (string, error) {
	safePath, err := toolutil.ValidatePathWritable(path)
	if err != nil {
		return "", fmt.Errorf("revert: invalid path: %w", err)
	}
	undoFile := snapshotUndoPath(safePath)
	preData, err := os.ReadFile(undoFile)
	if err != nil {
		return "", fmt.Errorf("revert: no snapshot for %s: %w", filepath.Base(safePath), err)
	}
	currentData, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("revert: read current file failed: %w", err)
	}
	mode := toolutil.GetFileMode(safePath)
	if err := os.WriteFile(safePath, preData, mode); err != nil {
		return "", fmt.Errorf("revert: write failed: %w", err)
	}
	newTag := execution.RecordSnapshot(safePath, string(preData))
	return renderRevertDiff(safePath, newTag, string(currentData), string(preData)), nil
}

func (t *EditTool) previewRevertSnapshot(path string) string {
	safePath, err := toolutil.ValidatePath(path)
	if err != nil {
		return ""
	}
	preData, err := os.ReadFile(snapshotUndoPath(safePath))
	if err != nil {
		return ""
	}
	currentData, err := os.ReadFile(safePath)
	if err != nil {
		return ""
	}
	return renderRevertDiff(safePath, "revert", string(currentData), string(preData))
}

func renderRevertDiff(path, tag, currentText, restoredText string) string {
	return diff.RenderTextChange(currentText, restoredText, diff.TextChangeOptions{
		Context:      diff.DefaultContext,
		Header:       diff.TagHeader(path, tag),
		NoChangeText: diff.NoChanges,
	})
}

func writeUndoSnapshot(pe preflightResult) error {
	undoFile := snapshotUndoPath(pe.safePath)
	dir := filepath.Dir(undoFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create undo snapshot directory: %w", err)
	}
	_ = os.Chmod(dir, 0o700)
	preEditContent := toolutil.RestoreLineEndings(pe.normalizedBefore, pe.lineEnding)
	if err := os.WriteFile(undoFile, []byte(preEditContent), 0o600); err != nil {
		return fmt.Errorf("write undo snapshot: %w", err)
	}
	if err := os.Chmod(undoFile, 0o600); err != nil {
		return fmt.Errorf("protect undo snapshot: %w", err)
	}
	pruneUndoSnapshots(dir, maxUndoSnapshots)
	return nil
}

func pruneUndoSnapshots(dir string, limit int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type snapshotFile struct {
		name    string
		modTime int64
	}
	files := make([]snapshotFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".pre-edit" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		_ = os.Chmod(path, 0o600)
		files = append(files, snapshotFile{name: path, modTime: info.ModTime().UnixNano()})
	}
	if limit < 0 || len(files) <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime == files[j].modTime {
			return files[i].name < files[j].name
		}
		return files[i].modTime < files[j].modTime
	})
	for _, file := range files[:len(files)-limit] {
		_ = os.Remove(file.name)
	}
}
