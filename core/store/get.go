package store

import "time"

func (s *Store) get(key string) (*Entry, bool) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if e.ExpireAt != nil && *e.ExpireAt <= time.Now().Unix() {
		s.mu.Lock()
		delete(s.data, key)
		s.mu.Unlock()
		return nil, false
	}

	return e, true
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
		return "not exist"
	}
	return e.Type.String()
}

// TODO: SEL
