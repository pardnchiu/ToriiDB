package store

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ValueType int

const (
	TypeJSON ValueType = iota
	TypeString
	TypeInt
	TypeFloat
	TypeBool
	TypeDate
)

type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Type      ValueType `json:"type"`
	CreatedAt int64     `json:"created_at"`
	UpdatedAt *int64    `json:"updated_at,omitempty"`
	ExpireAt  *int64    `json:"expire_at,omitempty"`
}

type SetFlag int

const (
	SetDefault SetFlag = iota // upsert
	SetNX                     // only if not exists
	SetXX                     // only if exists
)

func (s *Store) Set(key, value string, flag SetFlag, expireAt *int64) error {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now().Unix()
	existing, ok := db.data[key]

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

	var entry *Entry
	if ok {
		existing.Value = value
		existing.Type = detectType(value)
		existing.UpdatedAt = &now
		existing.ExpireAt = expireAt
		entry = existing
	} else {
		entry = &Entry{
			Key:       key,
			Value:     value,
			Type:      detectType(value),
			CreatedAt: now,
			ExpireAt:  expireAt,
		}
		db.data[key] = entry
	}

	path := db.filePath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return db.addToAOF("SET", key, value, expireAt)
}

// * use redis-fallback 3 layers store
func (d *db) filePath(key string) string {
	h := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return filepath.Join(d.dir, h[0:2], h[2:4], h[4:6], h+".json")
}

func detectType(value string) ValueType {
	if json.Valid([]byte(value)) {
		v := strings.TrimSpace(value)
		if (strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}")) ||
			(strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]")) {
			return TypeJSON
		}
	}

	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return TypeInt
	}

	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return TypeFloat
	}

	if value == "true" || value == "false" {
		return TypeBool
	}

	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return TypeDate
	}

	if _, err := time.Parse("2006-01-02", value); err == nil {
		return TypeDate
	}
	return TypeString
}

func (t ValueType) String() string {
	switch t {
	case TypeJSON:
		return "json"
	case TypeString:
		return "string"
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeBool:
		return "bool"
	case TypeDate:
		return "date"
	default:
		return "unknown"
	}
}
