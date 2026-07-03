package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"nekocode/common"
)

const maxInputHistoryEntries = 200

type inputHistoryFile struct {
	Entries []string `json:"entries"`
}

func inputHistoryPath() string {
	return filepath.Join(common.NekocodeHome(), "input_history.json")
}

func loadInputHistory() []string {
	data, err := os.ReadFile(inputHistoryPath())
	if err != nil {
		return nil
	}
	var f inputHistoryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil
	}
	return normalizeInputHistory(f.Entries)
}

func saveInputHistory(entries []string) error {
	f := inputHistoryFile{Entries: normalizeInputHistory(entries)}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return common.WriteFileWithDir(inputHistoryPath(), data, 0o600)
}

func appendInputHistory(entries []string, entry string) []string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return normalizeInputHistory(entries)
	}
	entries = append(normalizeInputHistory(entries), entry)
	return normalizeInputHistory(entries)
}

func normalizeInputHistory(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == entry {
			continue
		}
		out = append(out, entry)
	}
	if len(out) > maxInputHistoryEntries {
		out = out[len(out)-maxInputHistoryEntries:]
	}
	return out
}

func (m *Model) rememberInput(entry string) {
	history := appendInputHistory(m.Input.History(), entry)
	m.Input.SetHistory(history)
	_ = saveInputHistory(history)
}
