package store

import "time"

func (s *Store) Get(key string) (*Entry, bool) {
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

func (s *Store) Exist(key string) string {
	if e, ok := s.Get(key); !ok || e == nil {
		return "(integer) 0"
	}
	return "(integer) 1"
}

func (s *Store) Type(key string) string {
	e, ok := s.Get(key)
	if !ok || e == nil {
		return "(nil)"
	}
	return e.Type.String()
}

// TODO: SEL
