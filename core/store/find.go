package store

import (
	"sort"
	"sync"
	"time"

	"github.com/pardnchiu/ToriiDB/core/store/filter"
)

const sliceBlock = 1024

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

func sliceScan(keys []string, scan func(keys []string) []sortItem) []sortItem {
	n := len(keys)
	if n <= sliceBlock {
		return scan(keys)
	}

	chunks := (n + sliceBlock - 1) / sliceBlock
	shards := make([][]sortItem, chunks)
	var wg sync.WaitGroup

	for i := range chunks {
		start := i * sliceBlock
		end := start + sliceBlock
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(idx int, chunk []string) {
			defer wg.Done()
			shards[idx] = scan(chunk)
		}(i, keys[start:end])
	}

	wg.Wait()

	var merged []sortItem
	for _, s := range shards {
		merged = append(merged, s...)
	}
	return merged
}

func (c *core) Find(op filter.Operator, value string, limit int) []string {
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
			if filter.Match(e.Value, op, value) {
				out = append(out, sortItem{display: key, ts: entryTime(e)})
			}
		}
		return out
	})

	return sortAndCollect(items, limit)
}
