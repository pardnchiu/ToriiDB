package store

import "os"

func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, key := range keys {
		if _, ok := s.data[key]; !ok {
			continue
		}

		delete(s.data, key)
		os.Remove(filePath(key))
		s.addToAOF("DEL", key, "", nil)
		count++
	}

	return count
}
