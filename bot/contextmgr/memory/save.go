package memory

import (
	"strings"

	"nekocode/util/fs"
)

// Save writes the memory file to disk.
func (f *File) Save() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var b strings.Builder
	for _, key := range sectionOrder {
		header := sectionHeaders[key]
		content := f.getField(key)
		b.WriteString(header)
		b.WriteString("\n")
		if strings.TrimSpace(content) == "" {
			b.WriteString("\n")
		} else {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}
	return fs.WriteFileWithDir(f.path, []byte(b.String()), 0o644)
}
