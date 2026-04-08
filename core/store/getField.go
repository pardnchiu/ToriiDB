package store

import (
	"encoding/json"
	"strconv"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (s *Store) GetField(key string, subKeys []string) (string, bool) {
	entry, ok := s.Get(key)
	if !ok {
		return "", false
	}

	if entry.Type != TypeJSON {
		return "", false
	}

	var obj any
	if err := json.Unmarshal([]byte(entry.Value), &obj); err != nil {
		return "", false
	}

	val, ok := walkKeys(obj, subKeys)
	if !ok {
		return "", false
	}

	return utils.Vtoa(val), true
}

func walkKeys(obj any, subKeys []string) (any, bool) {
	current := obj
	for _, key := range subKeys {
		switch targetType := current.(type) {
		case map[string]any:
			sub, ok := targetType[key]
			if !ok {
				return nil, false
			}
			current = sub

		case []any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(targetType) {
				return nil, false
			}
			current = targetType[idx]

		default:
			return nil, false
		}
	}
	return current, true
}
