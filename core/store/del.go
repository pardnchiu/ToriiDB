package store

import (
	"fmt"
	"time"
)

func (c *core) Del(keys ...string) int {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	count := 0
	for _, key := range keys {
		if _, ok := db.data[key]; !ok {
			continue
		}

		delete(db.data, key)
		db.addToAOF("DEL", key, "", nil)
		count++
	}

	return count
}

func (c *core) DelField(key string, subKeys []string) error {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	entry, ok := db.data[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}
	if entry.Type != TypeJSON {
		return fmt.Errorf("not JSON type")
	}

	cached, cok := entry.parseAndCache()
	if !cok {
		return fmt.Errorf("json.Unmarshal failed")
	}
	obj, mok := cached.(map[string]any)
	if !mok {
		return fmt.Errorf("not a JSON object")
	}

	if err := walkKeysAndSet(obj, subKeys, nil); err != nil {
		return fmt.Errorf("walkKeysAndSet: %w", err)
	}

	if err := entry.setParsed(obj); err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	now := time.Now().Unix()
	entry.UpdatedAt = &now

	return db.addToAOF("SET", key, entry.Value(), entry.ExpireAt)
}
