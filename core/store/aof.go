package store

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

func (d *db) addToAOF(cmd, key, value string, expireAt *int64) error {
	if err := d.init(); err != nil {
		return err
	}

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

	if _, err := d.aof.WriteString(string(raw) + "\n"); err != nil {
		return err
	}

	return d.aof.Sync()
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
		case "SET":
			if e, ok := data[record.Key]; ok {
				e.Value = record.Value
				e.Type = detectType(record.Value)
				e.UpdatedAt = &record.Timestamp
				e.ExpireAt = record.ExpireAt
			} else {
				data[record.Key] = &Entry{
					Key:       record.Key,
					Value:     record.Value,
					Type:      detectType(record.Value),
					CreatedAt: record.Timestamp,
					ExpireAt:  record.ExpireAt,
				}
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
