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
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	expireAt := time.Now().Unix() + seconds
	e.ExpireAt = &expireAt

	return s.addToAOF("EXPIRE", key, fmt.Sprintf("%d", seconds), &expireAt)
}

func (s *Store) ExpireAt(key string, timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	e.ExpireAt = &timestamp

	return s.addToAOF("EXPIREAT", key, fmt.Sprintf("%d", timestamp), &timestamp)
}

func (s *Store) Persist(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return fmt.Errorf("not exist")
	}

	e.ExpireAt = nil

	return s.addToAOF("PERSIST", key, "", nil)
}
