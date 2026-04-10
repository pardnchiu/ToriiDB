package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
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

	d.aofLines++

	if err := d.aof.Sync(); err != nil {
		return err
	}

	live := len(d.data)
	if live >= compactMinLiveKeys && d.aofLines >= live*compactInflationRatio {
		return d.compact()
	}

	return nil
}

func replayAOF(path string) (map[string]*Entry, int, error) {
	data := make(map[string]*Entry)
	lines := 0
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, 0, nil
		}
		return nil, 0, err
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

		lines++

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

	return data, lines, scanner.Err()
}

const (
	compactInflationRatio = 2
	compactMinLiveKeys    = 1024
)

func (d *db) compact() error {
	if d.aof != nil {
		d.aof.Close()
		d.aof = nil
	}

	aofPath := filepath.Join(d.dir, "record.aof")

	if len(d.data) == 0 {
		os.Remove(aofPath)
		d.aofLines = 0
		return nil
	}

	now := time.Now().Unix()
	var buf []byte
	lines := 0

	for _, e := range d.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}

		record := AOFRecord{
			Timestamp: e.CreatedAt,
			Command:   "SET",
			Key:       e.Key,
			Value:     e.Value,
			ExpireAt:  e.ExpireAt,
		}

		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}

		buf = append(buf, raw...)
		buf = append(buf, '\n')
		lines++
	}

	if err := utils.WriteFile(aofPath, buf, 0644); err != nil {
		return err
	}

	d.aofLines = lines
	return nil
}
