package main

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	store := NewStore()
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("toriidb> ")

	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println()
			}
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Print("toriidb> ")
			continue
		}

		if strings.EqualFold(input, "quit") || strings.EqualFold(input, "exit") {
			break
		}

		fmt.Println(store.exec(input))
		fmt.Print("toriidb> ")
	}
}

const tempDir = "./temp"

type Entry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
}

type Store struct {
	mu   sync.RWMutex
	data map[string]*Entry
}

func NewStore() *Store {
	return &Store{
		data: make(map[string]*Entry),
	}
}

func (s *Store) exec(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}

	cmd := strings.ToUpper(parts[0])

	switch cmd {
	case "GET":
		if len(parts) != 2 {
			return "usage: GET <key>"
		}
		if e, ok := s.get(parts[1]); ok {
			return e.Value
		}
		return "(nil)"

	case "ADD":
		if len(parts) < 3 {
			return "usage: ADD <key> <value>"
		}
		key := parts[1]
		value := strings.Join(parts[2:], " ")

		if err := s.add(key, value); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "OK"

	default:
		return fmt.Sprintf("unknown: %s", cmd)
	}
}

func (s *Store) get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	return e, ok
}

func (s *Store) add(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := &Entry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now().Unix(),
	}

	s.data[key] = entry

	path := filePath(key)
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

	return nil
}

// * use redis-fallback 3 layers store
func filePath(key string) string {
	h := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return filepath.Join(tempDir, h[0:2], h[2:4], h[4:6], h+".json")
}
