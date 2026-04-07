package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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
		if e, ok := s.Get(parts[1]); ok {
			return e.Value
		}
		return "(nil)"

	case "EXIST":
		if len(parts) != 2 {
			return "usage: EXIST <key>"
		}
		return s.Exist(parts[1])

	case "TYPE":
		if len(parts) != 2 {
			return "usage: TYPE <key>"
		}
		return s.Type(parts[1])

	case "ADD":
		if len(parts) < 3 {
			return "usage: ADD <key> <value> [<seconds>]"
		}
		key := parts[1]
		value, expireAt := parseAddArgs(parts[2:])

		if err := s.Add(key, value, expireAt); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "OK"

	case "TTL":
		if len(parts) != 2 {
			return "usage: TTL <key>"
		}
		return fmt.Sprintf("(integer) %d", s.TTL(parts[1]))

	case "EXPIRE":
		if len(parts) != 3 {
			return "usage: EXPIRE <key> <seconds>"
		}
		sec, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || sec <= 0 {
			return "error: seconds must be a positive integer"
		}
		if err := s.Expire(parts[1], sec); err != nil {
			return err.Error()
		}
		return "OK"

	case "EXPIREAT":
		if len(parts) != 3 {
			return "usage: EXPIREAT <key> <timestamp>"
		}
		ts, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return "error: timestamp must be a valid integer"
		}
		if err := s.ExpireAt(parts[1], ts); err != nil {
			return err.Error()
		}
		return "OK"

	case "PERSIST":
		if len(parts) != 2 {
			return "usage: PERSIST <key>"
		}
		if err := s.Persist(parts[1]); err != nil {
			return err.Error()
		}
		return "OK"

	case "DEL":
		if len(parts) < 2 {
			return "usage: DEL <key> [key2] ..."
		}
		count := s.Del(parts[1:]...)
		return fmt.Sprintf("(integer) %d", count)

	default:
		return fmt.Sprintf("unknown: %s", cmd)
	}
}

func parseAddArgs(args []string) (string, *int64) {
	if len(args) >= 2 {
		sec, err := strconv.ParseInt(args[len(args)-1], 10, 64)
		if err == nil && sec > 0 {
			value := strings.Join(args[:len(args)-1], " ")
			expireAt := time.Now().Unix() + sec
			return value, &expireAt
		}
	}
	return strings.Join(args, " "), nil
}
