package store

import (
	"path"
	"sort"
	"time"
)

func (s *Store) Keys(pattern string) []string {
	db := s.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	var result []string
	for key, e := range db.data {
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
