package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const tempDir = "./temp"

type Entry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
}

type Store struct {
	mu   sync.RWMutex
	data map[string]*Entry
	aof  *os.File
}

type AOFRecord struct {
	Timestamp int64  `json:"ts"`
	Command   string `json:"cmd"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
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

	return &Store{
		data: data,
		aof:  file,
	}, nil
}

func (s *Store) Close() error {
	if s.aof != nil {
		return s.aof.Close()
	}
	return nil
}
