package store

import (
	"fmt"
	"time"

	"github.com/agenvoy/toriidb/core/utils"
)

func (c *core) IncrField(key string, subKeys []string, delta float64) (float64, error) {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	entry, ok := db.data[key]
	if !ok {
		return 0, fmt.Errorf("key not found: %s", key)
	}
	if entry.Type != TypeJSON {
		return 0, fmt.Errorf("not JSON type")
	}

	cached, cok := entry.parseAndCache()
	if !cok {
		return 0, fmt.Errorf("json.Unmarshal failed")
	}
	obj, mok := cached.(map[string]any)
	if !mok {
		return 0, fmt.Errorf("not a JSON object")
	}

	val, ok := utils.WalkKeys(obj, subKeys)
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

	if err := entry.setParsed(obj); err != nil {
		return 0, fmt.Errorf("json.Marshal: %w", err)
	}

	now := time.Now().Unix()
	entry.UpdatedAt = &now

	entryRaw, err := entry.JSON()
	if err != nil {
		return 0, fmt.Errorf("entry.JSON: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), entryRaw, 0644); err != nil {
		return 0, err
	}

	if err := db.addToAOF("SET", key, entry.Value(), entry.ExpireAt); err != nil {
		return 0, err
	}

	return result, nil
}
