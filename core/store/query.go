package store

import (
	"encoding/json"
	"time"

	"github.com/pardnchiu/ToriiDB/core/store/filter"
)

func (s *Store) Query(f filter.Filter, limit int) []string {
	db := s.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	keys := make([]string, 0, len(db.data))
	for k := range db.data {
		keys = append(keys, k)
	}

	items := sliceScan(keys, func(chunk []string) []sortItem {
		var out []sortItem
		for _, key := range chunk {
			e := db.data[key]
			if e.ExpireAt != nil && *e.ExpireAt <= now {
				continue
			}
			if e.Type != TypeJSON {
				continue
			}

			var obj any
			if err := json.Unmarshal([]byte(e.Value), &obj); err != nil {
				continue
			}

			if f.Match(obj) {
				out = append(out, sortItem{display: key + ": " + e.Value, ts: entryTime(e)})
			}
		}
		return out
	})

	return sortAndCollect(items, limit)
}
