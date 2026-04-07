package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const tempDir = "./temp"

type Store struct {
	mu      sync.RWMutex
	data    map[string]*Entry
	aof     *os.File
	cleanCh chan struct{}
}

type AOFRecord struct {
	Timestamp int64  `json:"ts"`
	Command   string `json:"cmd"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	ExpireAt  *int64 `json:"expire_at,omitempty"`
}

func New() (*Store, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	aofPath := filepath.Join(tempDir, "record.aof")
	file, err := os.OpenFile(aofPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open aof: %w", err)
	}

	data, err := replayAOF(aofPath)
	if err != nil {
		return nil, fmt.Errorf("replayAOF: %w", err)
	}

	s := &Store{
		data:    data,
		aof:     file,
		cleanCh: make(chan struct{}),
	}

	go s.cleanTimer(time.Minute)

	return s, nil
}

func (s *Store) Close() error {
	s.cleanCh <- struct{}{}

	if s.aof != nil {
		return s.aof.Close()
	}
	return nil
}

func (s *Store) cleanTimer(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-s.cleanCh:
			return
		case <-timer.C:
			s.cleanExpired()
			timer.Reset(interval)
		}
	}
}

func (s *Store) cleanExpired() {
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, e := range s.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			delete(s.data, key)
		}
	}
}
