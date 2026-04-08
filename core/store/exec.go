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
		key := parts[1]
		mainKey, subKeys := splitKey(key)
		if len(subKeys) > 0 {
			if val, ok := s.GetField(mainKey, subKeys); ok {
				return val
			}
			return "(nil)"
		}

		if e, ok := s.Get(mainKey); ok {
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

	case "SET":
		if len(parts) < 3 {
			return "usage: SET <key> <value> [NX|XX] [<seconds>]"
		}
		key := parts[1]
		mainKey, subKeys := splitKey(key)
		value, flag, expireAt := parseSetArgs(parts[2:])
		if len(subKeys) > 0 {
			if err := s.SetField(mainKey, subKeys, value, flag, expireAt); err != nil {
				return "(nil)"
			}
			return "OK"
		}

		if err := s.Set(key, value, flag, expireAt); err != nil {
			return "(nil)"
		}
		return "OK"

	case "TTL":
		if len(parts) != 2 {
			return "usage: TTL <key>"
		}
		ttl := s.TTL(parts[1])
		if ttl == -2 {
			return "(nil)"
		}
		return fmt.Sprintf("(integer) %d", ttl)

	case "EXPIRE":
		if len(parts) != 3 {
			return "usage: EXPIRE <key> <seconds>"
		}
		sec, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || sec <= 0 {
			return "error: seconds must be a positive integer"
		}
		if err := s.Expire(parts[1], sec); err != nil {
			return "(nil)"
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
			return "(nil)"
		}
		return "OK"

	case "PERSIST":
		if len(parts) != 2 {
			return "usage: PERSIST <key>"
		}
		if err := s.Persist(parts[1]); err != nil {
			return "(nil)"
		}
		return "OK"

	case "DEL":
		if len(parts) < 2 {
			return "usage: DEL <key> [key2] ..."
		}
		// * single key with dot notation → delete field
		if len(parts) == 2 {
			mainKey, subKeys := splitKey(parts[1])
			if len(subKeys) > 0 {
				if err := s.DelField(mainKey, subKeys); err != nil {
					return "(nil)"
				}
				return "OK"
			}
		}
		count := s.Del(parts[1:]...)
		return fmt.Sprintf("(integer) %d", count)

	case "KEYS":
		if len(parts) != 2 {
			return "usage: KEYS <pattern>"
		}
		keys := s.Keys(parts[1])
		if len(keys) == 0 {
			return "(empty list)"
		}
		var b strings.Builder
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "%d) %s", i+1, k)
		}
		return b.String()

	case "SELECT":
		if len(parts) != 2 {
			return "usage: SELECT <db> (0-15)"
		}
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			return "error: db index must be an integer"
		}
		if err := s.Select(index); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "OK"

	default:
		return fmt.Sprintf("unknown: %s", cmd)
	}
}

func parseSetArgs(args []string) (string, SetFlag, *int64) {
	flag := SetDefault
	var expireAt *int64
	end := len(args)

	if end >= 2 {
		sec, err := strconv.ParseInt(args[end-1], 10, 64)
		if err == nil && sec > 0 {
			ts := time.Now().Unix() + sec
			expireAt = &ts
			end--
		}
	}

	if end >= 2 {
		switch strings.ToUpper(args[end-1]) {
		case "NX":
			flag = SetNX
			end--
		case "XX":
			flag = SetXX
			end--
		}
	}

	return strings.Join(args[:end], " "), flag, expireAt
}

func splitKey(key string) (string, []string) {
	mainKey, subKeys, ok := strings.Cut(key, ".")
	if !ok || subKeys == "" {
		return mainKey, nil
	}

	var keys []string
	for subKey := range strings.SplitSeq(subKeys, ".") {
		if subKey != "" {
			keys = append(keys, subKey)
		}
	}
	return mainKey, keys
}
