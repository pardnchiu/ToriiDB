package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

const (
	compactInflationRatio = 2
	compactMinSize        = 1 << 20
)

func (d *db) addToAOF(cmd, key, value string, expireAt *int64) error {
	return d.addToAOFWithVector(cmd, key, value, expireAt, nil)
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

	if d.logSize >= max(d.snapSize, compactMinSize)*compactInflationRatio {
		return d.compact()
	}

	return nil
}

func replayInto(data map[string]*Entry, path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	for {
		line, readErr := reader.ReadBytes('\n')

		var record AOFRecord
		if len(line) == 0 || json.Unmarshal(line, &record) != nil {
			if readErr != nil {
				return eofOrErr(readErr)
			}
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

		if readErr != nil {
			return eofOrErr(readErr)
		}
	}
}

func eofOrErr(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
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
	if d.loadErr != nil {
		return d.loadErr
	}

	if len(d.data) == 0 && !go_pkg_filesystem_reader.IsDir(d.dir) {
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
		go_pkg_filesystem.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		go_pkg_filesystem.Remove(tmp)
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
