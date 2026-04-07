package store

func (s *Store) get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	return e, ok
}

// TODO: SEL
