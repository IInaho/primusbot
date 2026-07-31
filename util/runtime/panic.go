// Package runtime provides panic recovery and context helpers.
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"nekocode/util/fs"
)

const maxPanicLogs = 10

// WritePanicLog writes a panic recovery log to ~/.nekocode/logs/.
// Call from defer/recover blocks in both cmd and TUI.
func WritePanicLog(recoverVal any) {
	stack := string(debug.Stack())
	dir := fs.NekocodeLogDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create panic log dir: %v\n\nPANIC: %v\nStack:\n%s\n", err, recoverVal, stack)
		return
	}
	_ = os.Chmod(dir, 0o700)
	logPath := filepath.Join(dir, fmt.Sprintf("nekocode-panic-%d.log", time.Now().Unix()))
	if err := os.WriteFile(logPath, fmt.Appendf(nil, "PANIC: %v\n\nStack:\n%s", recoverVal, stack), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write panic log: %v\n", err)
		return
	}
	prunePanicLogs(dir, maxPanicLogs)
	fmt.Fprintf(os.Stderr, "\nPANIC: %v\nStack saved to %s\n", recoverVal, logPath)
}

func prunePanicLogs(dir string, limit int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "nekocode-panic-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	if limit < 0 || len(paths) <= limit {
		return
	}
	sort.Strings(paths)
	for _, path := range paths[:len(paths)-limit] {
		_ = os.Remove(path)
	}
}
