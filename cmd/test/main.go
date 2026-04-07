package main

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	store, err := NewStore()
	if err != nil {
		slog.Error("NewStore",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer store.Close()

	fmt.Print("toriidb> ")

	reader := bufio.NewReader(os.Stdin)
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

		if map[string]bool{
			"quit": true,
			"exit": true,
		}[input] {
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
	aof  *os.File
}

type AOFRecord struct {
	Timestamp int64  `json:"ts"`
	Command   string `json:"cmd"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
}

func NewStore() (*Store, error) {
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

func replayAOF(path string) (map[string]*Entry, error) {
	data := make(map[string]*Entry)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var record AOFRecord
		if json.Unmarshal([]byte(line), &record) != nil {
			continue
		}

		switch record.Command {
		case "ADD":
			data[record.Key] = &Entry{
				Key:       record.Key,
				Value:     record.Value,
				CreatedAt: record.Timestamp,
			}

		case "DEL":
			delete(data, record.Key)
		}
	}

	return data, scanner.Err()
}

func (s *Store) addToAOF(cmd, key, value string) error {
	rec := AOFRecord{
		Timestamp: time.Now().Unix(),
		Command:   cmd,
		Key:       key,
		Value:     value,
	}

	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	if _, err := s.aof.WriteString(string(raw) + "\n"); err != nil {
		return err
	}

	return s.aof.Sync()
}

func (s *Store) Close() error {
	if s.aof != nil {
		return s.aof.Close()
	}
	return nil
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

	case "DEL":
		if len(parts) < 2 {
			return "usage: DEL <key> [key2] ..."
		}
		count := s.del(parts[1:]...)
		return fmt.Sprintf("(integer) %d", count)

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

	return s.addToAOF("ADD", key, value)
}

func (s *Store) del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, key := range keys {
		if _, ok := s.data[key]; !ok {
			continue
		}

		delete(s.data, key)
		os.Remove(filePath(key))
		s.addToAOF("DEL", key, "")
		count++
	}

	return count
}

// * use redis-fallback 3 layers store
func filePath(key string) string {
	h := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	return filepath.Join(tempDir, h[0:2], h[2:4], h[4:6], h+".json")
}
