package store

import (
	"fmt"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
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
		go_pkg_filesystem.Remove(db.filePath(key))
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

	entryRaw, err := entry.JSON()
	if err != nil {
		return fmt.Errorf("entry.JSON: %w", err)
	}

	if err := go_pkg_filesystem.WriteFile(db.filePath(key), string(entryRaw), 0644); err != nil {
		return err
	}

	return db.addToAOF("SET", key, entry.Value(), entry.ExpireAt)
}
