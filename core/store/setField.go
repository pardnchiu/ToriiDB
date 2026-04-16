package store

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (c *core) SetField(key string, subKeys []string, value string, flag SetFlag, expireAt *int64) error {
	db := c.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now().Unix()
	oldVal, ok := db.data[key]

	switch flag {
	case SetNX:
		if ok {
			return fmt.Errorf("key already exists: %s", key)
		}
	case SetXX:
		if !ok {
			return fmt.Errorf("key not found: %s", key)
		}
	}

	var obj map[string]any
	if ok && oldVal.Type == TypeJSON {
		if cached, cok := oldVal.parseCached(); cok {
			if m, mok := cached.(map[string]any); mok {
				obj = m
			} else {
				obj = make(map[string]any)
			}
		} else {
			obj = make(map[string]any)
		}
	} else if ok {
		return fmt.Errorf("not JSON type")
	} else {
		obj = make(map[string]any)
	}

	if err := walkKeysAndSet(obj, subKeys, utils.Atov(value)); err != nil {
		return fmt.Errorf("walkKeysAndSet: %w", err)
	}

	var entry *Entry
	if ok {
		if err := oldVal.setParsed(obj); err != nil {
			return fmt.Errorf("oldVal.setParsed: %w", err)
		}
		oldVal.Type = TypeJSON
		oldVal.UpdatedAt = &now
		if expireAt != nil {
			oldVal.ExpireAt = expireAt
		}
		entry = oldVal
	} else {
		entry = &Entry{
			Key:       key,
			Type:      TypeJSON,
			CreatedAt: now,
			ExpireAt:  expireAt,
		}
		if err := entry.setParsed(obj); err != nil {
			return fmt.Errorf("entry.setParsed: %w", err)
		}
		db.data[key] = entry
	}

	entryRaw, err := entry.JSON()
	if err != nil {
		return fmt.Errorf("entry.JSON: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), entryRaw, 0644); err != nil {
		return err
	}

	return db.addToAOF("SET", key, entry.Value(), expireAt)
}

func walkKeysAndSet(obj map[string]any, fields []string, value any) error {
	newObj := any(obj)
	for _, field := range fields[:len(fields)-1] {
		switch newObjType := newObj.(type) {
		case map[string]any:
			next, ok := newObjType[field]
			if !ok {
				// * field not exist
				newMap := make(map[string]any)
				newObjType[field] = newMap
				newObj = newMap
			} else if nowMap, ok := next.(map[string]any); ok {
				// * field is map
				newObj = nowMap
			} else if arr, ok := next.([]any); ok {
				// * field is array
				newObj = arr
			} else {
				return fmt.Errorf("not json or array")
			}

		case []any:
			idx, err := strconv.Atoi(field)
			if err != nil || idx < 0 || idx >= len(newObjType) {
				return fmt.Errorf("invalid array index: %s", field)
			}
			newObj = newObjType[idx]

		default:
			return fmt.Errorf("failed to set field")
		}
	}

	last := fields[len(fields)-1]
	switch v := newObj.(type) {
	case map[string]any:
		if value == nil {
			delete(v, last)
		} else {
			v[last] = value
		}
	case []any:
		idx, err := strconv.Atoi(last)
		if err != nil || idx < 0 || idx >= len(v) {
			return fmt.Errorf("invalid array index: %s", last)
		}
		v[idx] = value
	default:
		return fmt.Errorf("failed to set field")
	}
	return nil
}
