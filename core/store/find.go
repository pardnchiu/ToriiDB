package store

import (
	"sort"
	"time"

	"github.com/pardnchiu/ToriiDB/core/store/filter"
)

type sortItem struct {
	display string
	ts      int64
}

func entryTime(e *Entry) int64 {
	if e.UpdatedAt != nil {
		return *e.UpdatedAt
	}
	return e.CreatedAt
}

func sortAndCollect(items []sortItem, limit int) []string {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ts != items[j].ts {
			return items[i].ts > items[j].ts
		}
		return items[i].display < items[j].display
	})

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	result := make([]string, len(items))
	for i, it := range items {
		result[i] = it.display
	}
	return result
}

func (s *Store) Find(op filter.Operator, value string, limit int) []string {
	db := s.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	var items []sortItem
	for key, e := range db.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}
		if filter.Match(e.Value, op, value) {
			items = append(items, sortItem{display: key, ts: entryTime(e)})
		}
	}

	return sortAndCollect(items, limit)
}
