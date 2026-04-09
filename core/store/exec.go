package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/ToriiDB/core/store/filter"
	"github.com/pardnchiu/ToriiDB/core/utils"
)

func (c *core) Exec(input string) string {
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
			if val, ok := c.GetField(mainKey, subKeys); ok {
				return val
			}
			return "(nil)"
		}

		if e, ok := c.Get(mainKey); ok {
			return e.Value
		}
		return "(nil)"

	case "EXIST":
		if len(parts) != 2 {
			return "usage: EXIST <key>"
		}
		mainKey, subKeys := splitKey(parts[1])
		if len(subKeys) > 0 {
			return c.ExistField(mainKey, subKeys)
		}
		return c.Exist(mainKey)

	case "TYPE":
		if len(parts) != 2 {
			return "usage: TYPE <key>"
		}
		mainKey, subKeys := splitKey(parts[1])
		if len(subKeys) > 0 {
			return c.TypeField(mainKey, subKeys)
		}
		return c.Type(mainKey)

	case "SET":
		if len(parts) < 3 {
			return "usage: SET <key> <value> [NX|XX] [<seconds>]"
		}
		key := parts[1]
		mainKey, subKeys := splitKey(key)
		value, flag, expireAt := parseSetArgs(parts[2:])
		if len(subKeys) > 0 {
			if err := c.SetField(mainKey, subKeys, value, flag, expireAt); err != nil {
				return "(nil)"
			}
			return "OK"
		}

		if err := c.Set(key, value, flag, expireAt); err != nil {
			return "(nil)"
		}
		return "OK"

	case "TTL":
		if len(parts) != 2 {
			return "usage: TTL <key>"
		}
		ttl := c.TTL(parts[1])
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
		if err := c.Expire(parts[1], sec); err != nil {
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
		if err := c.ExpireAt(parts[1], ts); err != nil {
			return "(nil)"
		}
		return "OK"

	case "PERSIST":
		if len(parts) != 2 {
			return "usage: PERSIST <key>"
		}
		if err := c.Persist(parts[1]); err != nil {
			return "(nil)"
		}
		return "OK"

	case "INCR":
		if len(parts) < 2 || len(parts) > 3 {
			return "usage: INCR <key> [delta]"
		}
		delta := 1.0
		if len(parts) == 3 {
			d, err := strconv.ParseFloat(parts[2], 64)
			if err != nil {
				return "error: delta must be a number"
			}
			delta = d
		}
		mainKey, subKeys := splitKey(parts[1])
		var result float64
		var err error
		if len(subKeys) > 0 {
			result, err = c.IncrField(mainKey, subKeys, delta)
		} else {
			result, err = c.Incr(mainKey, delta)
		}
		if err != nil {
			return "(nil)"
		}
		return utils.Vtoa(result)

	case "DEL":
		if len(parts) < 2 {
			return "usage: DEL <key> [key2] ..."
		}
		// * single key with dot notation → delete field
		if len(parts) == 2 {
			mainKey, subKeys := splitKey(parts[1])
			if len(subKeys) > 0 {
				if err := c.DelField(mainKey, subKeys); err != nil {
					return "(nil)"
				}
				return "OK"
			}
		}
		count := c.Del(parts[1:]...)
		return fmt.Sprintf("(integer) %d", count)

	case "KEYS":
		if len(parts) != 2 {
			return "usage: KEYS <pattern>"
		}
		keys := c.Keys(parts[1])
		return showList(keys)

	case "FIND":
		if len(parts) < 3 {
			return "usage: FIND <op> <value> [LIMIT <n>]"
		}
		op, ok := filter.AtoOperation(parts[1])
		if !ok {
			return "error: operator must be eq, ne, gt, gte/ge, lt, lte/le, or like"
		}
		tail, limit := parseLimit(parts[2:])
		value := strings.Join(tail, " ")
		return showList(c.Find(op, value, limit))

	case "QUERY":
		if len(parts) < 2 {
			return "usage: QUERY <expression> [LIMIT <n>]"
		}
		expr, limit := extractLimitFromInfix(parts[1:])
		f, err := filter.AtoFilter(expr)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return showList(c.Query(f, limit))

	case "SELECT":
		if len(parts) != 2 {
			return "usage: SELECT <db> (0-15)"
		}
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			return "error: db index must be an integer"
		}
		if err := c.Select(index); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "OK"

	default:
		return fmt.Sprintf("unknown: %s", cmd)
	}
}

func extractLimitFromInfix(tokens []string) (string, int) {
	n := len(tokens)
	if n >= 2 && strings.ToUpper(tokens[n-2]) == "LIMIT" {
		if v, err := strconv.Atoi(tokens[n-1]); err == nil && v > 0 {
			return strings.Join(tokens[:n-2], " "), v
		}
	}
	return strings.Join(tokens, " "), 0
}

func parseLimit(args []string) ([]string, int) {
	n := len(args)
	if n >= 2 && strings.ToUpper(args[n-2]) == "LIMIT" {
		if v, err := strconv.Atoi(args[n-1]); err == nil && v > 0 {
			return args[:n-2], v
		}
	}
	return args, 0
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

func showList(keys []string) string {
	if len(keys) == 0 {
		return "(empty list)"
	}

	var sb strings.Builder
	for i, key := range keys {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "%d) %s", i+1, key)
	}
	return sb.String()
}
