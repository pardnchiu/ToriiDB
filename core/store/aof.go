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

	line := append(raw, '\n')
	n, err := d.aof.Write(line)
	if err != nil {
		return err
	}

	d.aofSize += int64(n)

	if err := d.aof.Sync(); err != nil {
		return err
	}

	baseline := d.aofSizeBaseline
	if baseline < compactMinSize {
		baseline = compactMinSize
	}
	if d.aofSize >= baseline*compactInflationRatio {
		return d.compact()
	}

	return nil
}

func replayAOF(path string) (map[string]*Entry, int64, error) {
	data := make(map[string]*Entry)
	var size int64
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
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record AOFRecord
		if json.Unmarshal(line, &record) != nil {
			continue
		}

		size += int64(len(line) + 1)

		switch record.Command {
		case "SET":
			vType := detectType(record.Value)
			if e, ok := data[record.Key]; ok {
				e.setValue(record.Value)
				e.Type = vType
				e.UpdatedAt = &record.Timestamp
				e.ExpireAt = record.ExpireAt
			} else {
				e := &Entry{
					Key:       record.Key,
					Type:      vType,
					CreatedAt: record.Timestamp,
					ExpireAt:  record.ExpireAt,
				}
				e.setValue(record.Value)
				data[record.Key] = e
			}
			if vType == TypeJSON {
				if e, ok := data[record.Key]; ok {
					e.parseAndCache()
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

	return data, size, scanner.Err()
}

const (
	compactInflationRatio = 2
	compactMinSize        = 1 << 20
)

func (d *db) compact() error {
	if d.aof != nil {
		d.aof.Close()
		d.aof = nil
	}

	aofPath := filepath.Join(d.dir, "record.aof")

	if len(d.data) == 0 {
		os.Remove(aofPath)
		d.aofSize = 0
		d.aofSizeBaseline = 0
		return nil
	}

	now := time.Now().Unix()
	var buf []byte

	for _, e := range d.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}

		record := AOFRecord{
			Timestamp: e.CreatedAt,
			Command:   "SET",
			Key:       e.Key,
			Value:     e.Value(),
			ExpireAt:  e.ExpireAt,
		}

		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}

		buf = append(buf, raw...)
		buf = append(buf, '\n')
	}

	if err := utils.WriteFile(aofPath, buf, 0644); err != nil {
		return err
	}

	d.aofSize = int64(len(buf))
	d.aofSizeBaseline = d.aofSize
	return nil
}
