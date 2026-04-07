package store

import "os"

func (s *Store) Del(keys ...string) int {
	db := s.DB()
	db.mu.Lock()
	defer db.mu.Unlock()

	count := 0
	for _, key := range keys {
		if _, ok := db.data[key]; !ok {
			continue
		}

		delete(db.data, key)
		os.Remove(db.filePath(key))
		db.addToAOF("DEL", key, "", nil)
		count++
	}

	return count
}
