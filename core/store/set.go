package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

var ErrNoEmbedder = errors.New("no embedder: OPENAI_API_KEY not provided")

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
	Vector    []float32 `json:"vector,omitempty"`
	parsed    any
}

func (e *Entry) Value() string { return e.value }

func (e *Entry) setValue(v string) {
	e.value = v
	e.parsed = nil
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
		existing.Vector = nil
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

	return db.addToAOF("SET", key, value, expireAt)
}

func (c *core) SetVector(ctx context.Context, key, value string, flag SetFlag, expireAt *int64) error {
	if c.embedder == nil {
		slog.Debug("toriidb: SetVector without embedder",
			slog.String("key", key),
			slog.Int("db", c.db))
		return ErrNoEmbedder
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("vector text is empty")
	}

	if err := c.Set(key, value, flag, expireAt); err != nil {
		return err
	}

	dbIdx := c.db
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.attachVectorBG(dbIdx, key, value)
	}()
	return nil
}

func (c *core) attachVectorBG(dbIdx int, key, text string) {
	if c.embedder == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	d := c.dbs[dbIdx]
	d.ensureLoaded()
	model := c.embedder.model
	dim := c.embedder.dim

	d.mu.RLock()
	if vec, ok := d.getVector(model, dim, text); ok {
		d.mu.RUnlock()
		c.writeVectorToEntry(d, key, text, vec)
		return
	}
	d.mu.RUnlock()

	vec, err := c.embedder.embed(ctx, text)
	if err != nil {
		return
	}

	d.mu.Lock()
	_ = d.putVector(model, dim, text, vec)
	d.mu.Unlock()

	c.writeVectorToEntry(d, key, text, vec)
}

func (c *core) writeVectorToEntry(d *db, key, text string, vec []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.data[key]
	if !ok {
		return
	}
	if entry.Value() != text {
		return
	}

	entry.Vector = vec

	_ = d.addToAOFWithVector("SET", key, entry.Value(), entry.ExpireAt, vec)
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
