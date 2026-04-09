package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (c *core) TTL(key string) int64 {
	e, ok := c.Get(key)
	if !ok {
		return -2
	}

	if e.ExpireAt == nil {
		return -1
	}

	remaining := *e.ExpireAt - time.Now().Unix()
	if remaining <= 0 {
		return -2
	}

	return remaining
}

func (c *core) Expire(key string, seconds int64) error {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	e, ok := db.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	expireAt := time.Now().Unix() + seconds
	e.ExpireAt = &expireAt

	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), raw, 0644); err != nil {
		return err
	}

	return db.addToAOF("EXPIRE", key, fmt.Sprintf("%d", seconds), &expireAt)
}

func (c *core) ExpireAt(key string, timestamp int64) error {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	e, ok := db.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	e.ExpireAt = &timestamp

	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), raw, 0644); err != nil {
		return err
	}

	return db.addToAOF("EXPIREAT", key, fmt.Sprintf("%d", timestamp), &timestamp)
}

func (c *core) Persist(key string) error {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	e, ok := db.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	e.ExpireAt = nil

	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), raw, 0644); err != nil {
		return err
	}

	return db.addToAOF("PERSIST", key, "", nil)
}
