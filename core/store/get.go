package store

import (
	"time"

	"github.com/agenvoy/toriidb/core/utils"
)

func (c *core) Get(key string) (*Entry, bool) {
	db := c.DB()
	db.mu.RLock()
	e, ok := db.data[key]
	db.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if e.ExpireAt != nil && *e.ExpireAt <= time.Now().Unix() {
		db.mu.Lock()
		delete(db.data, key)
		db.mu.Unlock()
		return nil, false
	}

	return e, true
}

func (c *core) GetField(key string, subKeys []string) (string, bool) {
	db := c.DB()
	db.mu.RLock()
	defer db.mu.RUnlock()

	entry, ok := db.data[key]
	if !ok {
		return "", false
	}
	if entry.ExpireAt != nil && *entry.ExpireAt <= time.Now().Unix() {
		return "", false
	}

	obj, ok := entry.cached()
	if !ok {
		return "", false
	}

	val, ok := utils.WalkKeys(obj, subKeys)
	if !ok {
		return "", false
	}

	return utils.Vtoa(val), true
}

func (c *core) Exist(key string) string {
	if e, ok := c.Get(key); !ok || e == nil {
		return "(integer) 0"
	}
	return "(integer) 1"
}

func (c *core) ExistField(key string, subKeys []string) string {
	if _, ok := c.GetField(key, subKeys); !ok {
		return "(integer) 0"
	}
	return "(integer) 1"
}

func (c *core) Type(key string) string {
	e, ok := c.Get(key)
	if !ok || e == nil {
		return "(nil)"
	}
	return e.Type.String()
}

func (c *core) TypeField(key string, subKeys []string) string {
	val, ok := c.GetField(key, subKeys)
	if !ok {
		return "(nil)"
	}

	return detectType(val).String()
}
