package store

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (s *Store) Incr(key string, delta float64) (float64, error) {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	entry, ok := db.data[key]
	if !ok {
		return 0, fmt.Errorf("key not found: %s", key)
	}

	switch entry.Type {
	case TypeInt, TypeFloat:
	default:
		return 0, fmt.Errorf("not number type")
	}

	num, err := strconv.ParseFloat(entry.Value, 64)
	if err != nil {
		return 0, fmt.Errorf("strconv.ParseFloat: %w", err)
	}

	result := num + delta
	now := time.Now().Unix()
	entry.Value = utils.Vtoa(result)
	entry.Type = detectType(entry.Value)
	entry.UpdatedAt = &now

	raw, err := json.Marshal(entry)
	if err != nil {
		return 0, fmt.Errorf("json.Marshal: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), raw, 0644); err != nil {
		return 0, err
	}

	if err := db.addToAOF("SET", key, entry.Value, entry.ExpireAt); err != nil {
		return 0, err
	}

	return result, nil
}
