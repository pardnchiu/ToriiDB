package store

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type FindOperation int

const (
	FindEqualTo FindOperation = iota
	FindGreaterThan
	FindGreaterThanOrEqualTo
	FindLessThan
	FindLessThanOrEqualTo
	FindLIKE
)

func ParseFindOp(s string) (FindOperation, bool) {
	switch strings.ToUpper(s) {
	case "EQ", "=":
		return FindEqualTo, true

	case "GT", ">":
		return FindGreaterThan, true

	case "GE", ">=":
		return FindGreaterThanOrEqualTo, true

	case "LT", "<":
		return FindLessThan, true

	case "LE", "<=":
		return FindLessThanOrEqualTo, true

	case "LIKE":
		return FindLIKE, true

	default:
		return 0, false
	}
}

type sortItem struct {
	display string
	ts      int64
}

func entryTime(e *Entry) int64 {
	if e.UpdatedAt != nil {
		return *e.UpdatedAt
	}
	return e.CreatedAt
}

func sortAndCollect(items []sortItem, limit int) []string {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ts != items[j].ts {
			return items[i].ts > items[j].ts
		}
		return items[i].display < items[j].display
	})

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	result := make([]string, len(items))
	for i, it := range items {
		result[i] = it.display
	}
	return result
}

func (s *Store) Find(op FindOperation, value string, limit int) []string {
	db := s.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	var items []sortItem
	for key, e := range db.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}
		if matchValue(e.Value, op, value) {
			items = append(items, sortItem{display: key, ts: entryTime(e)})
		}
	}

	return sortAndCollect(items, limit)
}

func matchValue(stored string, op FindOperation, target string) bool {
	switch op {
	case FindEqualTo:
		return stored == target

	case FindGreaterThan:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv > tv

	case FindGreaterThanOrEqualTo:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv >= tv

	case FindLessThan:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv < tv

	case FindLessThanOrEqualTo:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv <= tv

	case FindLIKE:
		return matchText(stored, target)

	default:
		return false
	}
}

func parseNum(a, b string) (float64, float64, bool) {
	av, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return 0, 0, false
	}

	bv, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return 0, 0, false
	}
	return av, bv, true
}

func matchText(stored, pattern string) bool {
	prefix := strings.HasPrefix(pattern, "*")
	suffix := strings.HasSuffix(pattern, "*")
	core := strings.Trim(pattern, "*")

	if core == "" {
		return true
	}

	switch {
	case prefix && suffix:
		return strings.Contains(stored, core)

	case prefix:
		return strings.HasSuffix(stored, core)

	case suffix:
		return strings.HasPrefix(stored, core)

	default:
		return strings.Contains(stored, core)
	}
}
