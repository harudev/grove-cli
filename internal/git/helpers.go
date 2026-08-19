package git

import (
	"os"
	"path/filepath"
)

// openFileAppend opens a file for appending (creates if needed).
func openFileAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
