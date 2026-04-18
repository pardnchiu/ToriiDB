package store

import (
	"path"
	"sort"
	"time"
)

func (c *core) Keys(pattern string) []string {
	db := c.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	var result []string
	for key, e := range db.data {
		if isInternal(key) {
			continue
		}
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}

		matched, err := path.Match(pattern, key)
		if err != nil {
			continue
		}
		if matched {
			result = append(result, key)
		}
	}

	sort.Strings(result)
	return result
}
