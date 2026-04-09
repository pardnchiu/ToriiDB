package store

import (
	"encoding/json"
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
		// * value is json
		if err := json.Unmarshal([]byte(oldVal.Value), &obj); err != nil {
			obj = make(map[string]any)
		}
	} else if ok {
		return fmt.Errorf("not JSON type")
	} else {
		// * value not exist, create new
		obj = make(map[string]any)
	}

	if err := walkKeysAndSet(obj, subKeys, utils.Atov(value)); err != nil {
		return fmt.Errorf("walkKeysAndSet: %w", err)
	}

	raw, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	newVal := string(raw)
	var entry *Entry
	if ok {
		oldVal.Value = newVal
		oldVal.Type = TypeJSON
		oldVal.UpdatedAt = &now
		if expireAt != nil {
			oldVal.ExpireAt = expireAt
		}
		entry = oldVal
	} else {
		entry = &Entry{
			Key:       key,
			Value:     newVal,
			Type:      TypeJSON,
			CreatedAt: now,
			ExpireAt:  expireAt,
		}
		db.data[key] = entry
	}

	entryRaw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), entryRaw, 0644); err != nil {
		return err
	}

	return db.addToAOF("SET", key, newVal, expireAt)
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
