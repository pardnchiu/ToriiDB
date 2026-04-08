package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (d *db) compact() error {
	if d.aof != nil {
		d.aof.Close()
		d.aof = nil
	}

	if len(d.data) == 0 {
		os.Remove(filepath.Join(d.dir, "record.aof"))
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

	return utils.WriteFile(filepath.Join(d.dir, "record.aof"), buf, 0644)
}
