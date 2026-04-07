package store

import (
	"fmt"
	"time"
)

func (s *Store) TTL(key string) int64 {
	e, ok := s.Get(key)
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

func (s *Store) Expire(key string, seconds int64) error {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	e, ok := db.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	expireAt := time.Now().Unix() + seconds
	e.ExpireAt = &expireAt

	return db.addToAOF("EXPIRE", key, fmt.Sprintf("%d", seconds), &expireAt)
}

func (s *Store) ExpireAt(key string, timestamp int64) error {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	e, ok := db.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	e.ExpireAt = &timestamp

	return db.addToAOF("EXPIREAT", key, fmt.Sprintf("%d", timestamp), &timestamp)
}

func (s *Store) Persist(key string) error {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	e, ok := db.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	e.ExpireAt = nil

	return db.addToAOF("PERSIST", key, "", nil)
}
