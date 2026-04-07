package store

func (s *Store) get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	return e, ok
}

func (s *Store) EXISTS(key string) string {
	if e, ok := s.get(key); !ok || e == nil {
		return "(integer) 0"
	}
	return "(integer) 1"
}

func (s *Store) TYPE(key string) string {
	e, ok := s.get(key)
	if !ok || e == nil {
		return "none"
	}
	return e.Type.String()
}

// TODO: SEL
