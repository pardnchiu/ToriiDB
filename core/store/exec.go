package store

import (
	"fmt"
	"strings"
)

func (s *Store) Exec(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}

	cmd := strings.ToUpper(parts[0])

	switch cmd {
	case "GET":
		if len(parts) != 2 {
			return "usage: GET <key>"
		}
		if e, ok := s.get(parts[1]); ok {
			return e.Value
		}
		return "(nil)"

	case "EXIST":
		if len(parts) != 2 {
			return "usage: EXIST <key>"
		}
		return s.EXISTS(parts[1])

	case "TYPE":
		if len(parts) != 2 {
			return "usage: TYPE <key>"
		}
		return s.TYPE(parts[1])

	case "ADD":
		if len(parts) < 3 {
			return "usage: ADD <key> <value>"
		}
		key := parts[1]
		value := strings.Join(parts[2:], " ")

		if err := s.add(key, value); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "OK"

	case "DEL":
		if len(parts) < 2 {
			return "usage: DEL <key> [key2] ..."
		}
		count := s.del(parts[1:]...)
		return fmt.Sprintf("(integer) %d", count)

	default:
		return fmt.Sprintf("unknown: %s", cmd)
	}
}
