package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (s *Store) DelField(key string, subKeys []string) error {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	entry, ok := db.data[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}
	if entry.Type != TypeJSON {
		return fmt.Errorf("not JSON type")
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(entry.Value), &obj); err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}

	if err := walkKeysAndSet(obj, subKeys, nil); err != nil {
		return fmt.Errorf("walkKeysAndSet: %w", err)
	}

	raw, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	now := time.Now().Unix()
	newVal := string(raw)
	entry.Value = newVal
	entry.UpdatedAt = &now

	entryRaw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), entryRaw, 0644); err != nil {
		return err
	}

	return db.addToAOF("SET", key, newVal, entry.ExpireAt)
}
