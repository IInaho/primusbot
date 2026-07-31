package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrunePanicLogs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"nekocode-panic-1.log",
		"nekocode-panic-2.log",
		"nekocode-panic-3.log",
		"nekocode-debug.log",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	prunePanicLogs(dir, 2)

	if _, err := os.Stat(filepath.Join(dir, "nekocode-panic-1.log")); !os.IsNotExist(err) {
		t.Fatalf("oldest panic log was not removed: %v", err)
	}
	for _, name := range []string{"nekocode-panic-2.log", "nekocode-panic-3.log", "nekocode-debug.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was removed: %v", name, err)
		}
	}
}
