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

func (s *Store) Find(op FindOperation, value string) []string {
	db := s.DB()
	now := time.Now().Unix()

	db.mu.RLock()
	defer db.mu.RUnlock()

	var result []string
	for key, e := range db.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}
		if matchValue(e.Value, op, value) {
			result = append(result, key)
		}
	}

	sort.Strings(result)
	return result
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
