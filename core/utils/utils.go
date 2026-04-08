package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFile(path string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("os.Rename: %w", err)
	}

	return nil
}
