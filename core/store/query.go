package store

import (
	"time"

	"github.com/pardnchiu/ToriiDB/core/store/filter"
)

func (c *core) Query(f filter.Filter, limit int) []string {
	db := c.DB()
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
			obj, ok := e.parseCached()
			if !ok {
				continue
			}

			if f.Match(obj) {
				out = append(out, sortItem{display: key + ": " + e.Value(), ts: entryTime(e)})
			}
		}
		return out
	})

	return sortAndCollect(items, limit)
}
