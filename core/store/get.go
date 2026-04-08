package store

import (
	"encoding/json"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (s *Store) Get(key string) (*Entry, bool) {
	db := s.DB()
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

	val, ok := utils.WalkKeys(obj, subKeys)
	if !ok {
		return "", false
	}

	return utils.Vtoa(val), true
}

func (s *Store) Exist(key string) string {
	if e, ok := s.Get(key); !ok || e == nil {
		return "(integer) 0"
	}
	return "(integer) 1"
}

func (s *Store) ExistField(key string, subKeys []string) string {
	if _, ok := s.GetField(key, subKeys); !ok {
		return "(integer) 0"
	}
	return "(integer) 1"
}

func (s *Store) Type(key string) string {
	e, ok := s.Get(key)
	if !ok || e == nil {
		return "(nil)"
	}
	return e.Type.String()
}

func (s *Store) TypeField(key string, subKeys []string) string {
	val, ok := s.GetField(key, subKeys)
	if !ok {
		return "(nil)"
	}

	return detectType(val).String()
}
