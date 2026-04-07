package store

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (s *Store) add(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := &Entry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now().Unix(),
	}

	s.data[key] = entry

	path := filePath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return s.addToAOF("ADD", key, value)
}

// * use redis-fallback 3 layers store
func filePath(key string) string {
	h := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return filepath.Join(tempDir, h[0:2], h[2:4], h[4:6], h+".json")
}

// TODO: SET
