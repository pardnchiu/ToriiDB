package store

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
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
	Key       string `json:"key"`
	value     string
	Type      ValueType `json:"type"`
	CreatedAt int64     `json:"created_at"`
	UpdatedAt *int64    `json:"updated_at,omitempty"`
	ExpireAt  *int64    `json:"expire_at,omitempty"`
	parsed    any
}

func (e *Entry) Value() string { return e.value }

func (e *Entry) setValue(v string) {
	e.value = v
	e.parsed = nil
}

func (e *Entry) JSON() ([]byte, error) {
	type data struct {
		Key       string    `json:"key"`
		Value     string    `json:"value"`
		Type      ValueType `json:"type"`
		CreatedAt int64     `json:"created_at"`
		UpdatedAt *int64    `json:"updated_at,omitempty"`
		ExpireAt  *int64    `json:"expire_at,omitempty"`
	}
	return json.Marshal(data{
		Key:       e.Key,
		Value:     e.value,
		Type:      e.Type,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		ExpireAt:  e.ExpireAt,
	})
}

func (e *Entry) parseAndCache() (any, bool) {
	if e.Type != TypeJSON {
		return nil, false
	}
	if e.parsed != nil {
		return e.parsed, true
	}
	var obj any
	if json.Unmarshal([]byte(e.value), &obj) != nil {
		return nil, false
	}
	e.parsed = obj
	return obj, true
}

func (e *Entry) cached() (any, bool) {
	if e.Type != TypeJSON || e.parsed == nil {
		return nil, false
	}
	return e.parsed, true
}

func (e *Entry) setParsed(obj any) error {
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	e.value = string(raw)
	e.parsed = obj
	return nil
}

type SetFlag int

const (
	SetDefault SetFlag = iota // upsert
	SetNX                     // only if not exists
	SetXX                     // only if exists
)

func (c *core) Set(key, value string, flag SetFlag, expireAt *int64) error {
	db := c.DB()
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
	vType := detectType(value)
	if ok {
		existing.setValue(value)
		existing.Type = vType
		existing.UpdatedAt = &now
		existing.ExpireAt = expireAt
		entry = existing
	} else {
		entry = &Entry{
			Key:       key,
			Type:      vType,
			CreatedAt: now,
			ExpireAt:  expireAt,
		}
		entry.setValue(value)
		db.data[key] = entry
	}

	if vType == TypeJSON {
		entry.parseAndCache()
	}

	raw, err := entry.JSON()
	if err != nil {
		return fmt.Errorf("entry.JSON: %w", err)
	}

	if err := utils.WriteFile(db.filePath(key), raw, 0644); err != nil {
		return err
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
