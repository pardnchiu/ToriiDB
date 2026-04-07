package store

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

func (s *Store) addToAOF(cmd, key, value string, expireAt *int64) error {
	record := AOFRecord{
		Timestamp: time.Now().Unix(),
		Command:   cmd,
		Key:       key,
		Value:     value,
		ExpireAt:  expireAt,
	}

	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if _, err := s.aof.WriteString(string(raw) + "\n"); err != nil {
		return err
	}

	return s.aof.Sync()
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
				Type:      detectType(record.Value),
				CreatedAt: record.Timestamp,
				ExpireAt:  record.ExpireAt,
			}

		case "DEL":
			delete(data, record.Key)

		case "EXPIRE", "EXPIREAT":
			if e, ok := data[record.Key]; ok {
				e.ExpireAt = record.ExpireAt
			}

		case "PERSIST":
			if e, ok := data[record.Key]; ok {
				e.ExpireAt = nil
			}
		}
	}

	return data, scanner.Err()
}
