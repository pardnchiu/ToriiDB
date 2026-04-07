package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	tempDir = "./temp"
	maxDB   = 16
)

type db struct {
	mu   sync.RWMutex
	dir  string
	data map[string]*Entry
	aof  *os.File
}

type Store struct {
	dbs     [maxDB]*db
	db      int
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
	s := &Store{
		cleanCh: make(chan struct{}),
	}

	for i := range maxDB {
		d, err := new(i)
		if err != nil {
			return nil, fmt.Errorf("db_%d: %w", i, err)
		}
		s.dbs[i] = d
	}

	go s.cleanTimer(time.Minute)

	return s, nil
}

func new(index int) (*db, error) {
	dir := filepath.Join(tempDir, fmt.Sprintf("db_%d", index))
	aofPath := filepath.Join(dir, "record.aof")

	data, err := replayAOF(aofPath)
	if err != nil {
		return nil, fmt.Errorf("replayAOF: %w", err)
	}

	return &db{
		dir:  dir,
		data: data,
	}, nil
}

func (d *db) init() error {
	if d.aof != nil {
		return nil
	}

	if err := os.MkdirAll(d.dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	aofPath := filepath.Join(d.dir, "record.aof")
	file, err := os.OpenFile(aofPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open aof: %w", err)
	}

	d.aof = file
	return nil
}

func (s *Store) DB() *db {
	return s.dbs[s.db]
}

func (s *Store) Current() int {
	return s.db
}

func (s *Store) Select(index int) error {
	if index < 0 || index >= maxDB {
		return fmt.Errorf("invalid db index: %d (0-%d)", index, maxDB-1)
	}
	s.db = index
	return nil
}

func (s *Store) Close() error {
	s.cleanCh <- struct{}{}

	var firstErr error
	for _, d := range s.dbs {
		if d == nil {
			continue
		}

		d.mu.Lock()
		if d.aof != nil {
			if err := d.compact(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		d.mu.Unlock()
	}
	return firstErr
}

func (s *Store) cleanTimer(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-s.cleanCh:
			return
		case <-timer.C:
			for _, d := range s.dbs {
				d.cleanExpired()
			}
			timer.Reset(interval)
		}
	}
}

func (d *db) cleanExpired() {
	now := time.Now().Unix()

	d.mu.Lock()
	defer d.mu.Unlock()

	for key, e := range d.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			delete(d.data, key)
		}
	}
}
