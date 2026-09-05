package store

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

const (
	compactInflationRatio = 2
	compactMinSize        = 1 << 20
)

func (d *db) addToAOF(cmd, key, value string, expireAt *int64) error {
	return d.writeAOF(AOFRecord{
		Timestamp: time.Now().Unix(),
		Command:   cmd,
		Key:       key,
		Value:     value,
		ExpireAt:  expireAt,
	})
}

func (d *db) addToAOFWithVector(cmd, key, value string, expireAt *int64, vec []float32) error {
	var vecPtr *string
	if len(vec) > 0 {
		encoded := encodeVector(vec)
		vecPtr = &encoded
	}
	return d.writeAOF(AOFRecord{
		Timestamp: time.Now().Unix(),
		Command:   cmd,
		Key:       key,
		Value:     value,
		ExpireAt:  expireAt,
		Vector:    vecPtr,
	})
}

func (d *db) writeAOF(record AOFRecord) error {
	if err := d.init(); err != nil {
		return err
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

	d.logSize += int64(n)

	if err := d.aof.Sync(); err != nil {
		return err
	}

	return d.maybeCompact()
}

func (d *db) maybeCompact() error {
	baseline := max(d.snapSize, compactMinSize)
	if d.logSize >= baseline*compactInflationRatio {
		return d.compact()
	}
	return nil
}

func replayInto(data map[string]*Entry, path string) error {
	if path == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
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

		switch record.Command {
		case "SET":
			vType := detectType(record.Value)
			var vec []float32
			if record.Vector != nil {
				decoded, err := decodeVector(*record.Vector)
				if err != nil {
					continue
				}
				vec = decoded
			}

			if e, ok := data[record.Key]; ok {
				e.setValue(record.Value)
				e.Type = vType
				e.UpdatedAt = &record.Timestamp
				e.ExpireAt = record.ExpireAt
				e.Vector = vec
			} else {
				e := &Entry{
					Key:       record.Key,
					Type:      vType,
					CreatedAt: record.Timestamp,
					ExpireAt:  record.ExpireAt,
					Vector:    vec,
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

	return scanner.Err()
}

func replayFile(path string) (map[string]*Entry, error) {
	data := make(map[string]*Entry)
	if err := replayInto(data, path); err != nil {
		return nil, err
	}
	return data, nil
}

func (d *db) serialize() ([]byte, error) {
	now := time.Now().Unix()
	var buf []byte

	for _, e := range d.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}

		var vector *string
		if len(e.Vector) > 0 {
			encoded := encodeVector(e.Vector)
			vector = &encoded
		}

		record := AOFRecord{
			Timestamp: e.CreatedAt,
			Command:   "SET",
			Key:       e.Key,
			Value:     e.Value(),
			ExpireAt:  e.ExpireAt,
			Vector:    vector,
		}

		raw, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}

		buf = append(buf, raw...)
		buf = append(buf, '\n')
	}

	return buf, nil
}

func (d *db) compact() error {
	if len(d.data) == 0 && !dirExists(d.dir) {
		return nil
	}

	n := d.num
	dst := snapPath(d.dir, n)
	tmp := dst + ".tmp"

	buf, err := d.serialize()
	if err != nil {
		return err
	}

	if err := writeSync(tmp, buf); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := syncDir(d.dir); err != nil {
		return err
	}

	lg, err := os.OpenFile(logPath(d.dir, n+1), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	old := d.aof
	d.aof = lg
	d.num = n + 1
	d.logSize = 0
	d.snapSize = int64(len(buf))
	if old != nil {
		old.Close()
	}

	d.gen = bumpGen(d.dir)
	go gcOlderThan(d.dir, n)

	return nil
}

func writeSync(path string, buf []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := file.Write(buf); err != nil {
		file.Close()
		return err
	}

	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}

func syncDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
