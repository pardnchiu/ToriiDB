package store

import (
	"encoding/json"
	"time"

	"github.com/pardnchiu/ToriiDB/core/store/filter"
)

func (s *Store) Query(filter filter.Filter, limit int) []string {
	db := s.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	var items []sortItem
	for key, e := range db.data {
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

		if filter.Match(obj) {
			items = append(items, sortItem{display: key + ": " + e.Value, ts: entryTime(e)})
		}
	}

	return sortAndCollect(items, limit)
}
