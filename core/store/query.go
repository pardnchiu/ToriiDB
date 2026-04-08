package store

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (s *Store) Query(field string, op FindOperation, value string, limit int) []string {
	var subKeys []string
	for part := range strings.SplitSeq(field, ".") {
		if part != "" {
			subKeys = append(subKeys, part)
		}
	}

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

		val, ok := utils.WalkKeys(obj, subKeys)
		if !ok {
			continue
		}

		if matchValue(utils.Vtoa(val), op, value) {
			items = append(items, sortItem{display: key + ": " + e.Value, ts: entryTime(e)})
		}
	}

	return sortAndCollect(items, limit)
}
