package store

import (
	"encoding/json"
	"path/filepath"
	"time"
)

func (d *db) compact() error {
	if d.aof == nil {
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
			Value:     e.Value,
			ExpireAt:  e.ExpireAt,
		}

		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}

		buf = append(buf, raw...)
		buf = append(buf, '\n')
	}

	d.aof.Close()
	d.aof = nil

	return writeFile(filepath.Join(d.dir, "record.aof"), buf, 0644)
}
