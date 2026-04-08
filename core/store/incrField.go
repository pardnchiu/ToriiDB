package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (s *Store) IncrField(key string, subKeys []string, delta float64) (float64, error) {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	entry, ok := db.data[key]
	if !ok {
		return 0, fmt.Errorf("key not found: %s", key)
	}
	if entry.Type != TypeJSON {
		return 0, fmt.Errorf("not JSON type")
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(entry.Value), &obj); err != nil {
		return 0, fmt.Errorf("json.Unmarshal: %w", err)
	}

	val, ok := walkKeys(obj, subKeys)
	if !ok {
		return 0, fmt.Errorf("field not found")
	}

	num, ok := utils.Vtof(val)
	if !ok {
		return 0, fmt.Errorf("field is not a number")
	}

	result := num + delta
	if err := walkKeysAndSet(obj, subKeys, result); err != nil {
		return 0, fmt.Errorf("walkKeysAndSet: %w", err)
	}

	raw, err := json.Marshal(obj)
	if err != nil {
		return 0, fmt.Errorf("json.Marshal: %w", err)
	}

	now := time.Now().Unix()
	newVal := string(raw)
	entry.Value = newVal
	entry.UpdatedAt = &now

	entryRaw, err := json.Marshal(entry)
	if err != nil {
		return 0, fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), entryRaw, 0644); err != nil {
		return 0, err
	}

	if err := db.addToAOF("SET", key, newVal, entry.ExpireAt); err != nil {
		return 0, err
	}

	return result, nil
}
