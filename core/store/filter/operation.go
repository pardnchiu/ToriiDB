package filter

import (
	"strconv"
	"strings"
)

type Operator int

const (
	EqualTo Operator = iota
	GreaterThan
	GreaterThanOrEqualTo
	LessThan
	LessThanOrEqualTo
	NotEqualTo
	Like
)

func AtoOperation(s string) (Operator, bool) {
	switch strings.ToUpper(s) {
	case "EQ", "=":
		return EqualTo, true

	case "GT", ">":
		return GreaterThan, true

	case "GTE", "GE", ">=":
		return GreaterThanOrEqualTo, true

	case "LT", "<":
		return LessThan, true

	case "LTE", "LE", "<=":
		return LessThanOrEqualTo, true

	case "NE", "!=":
		return NotEqualTo, true

	case "LIKE":
		return Like, true

	default:
		return 0, false
	}
}

func Match(stored string, op Operator, target string) bool {
	switch op {
	case EqualTo:
		return stored == target

	case GreaterThan:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv > tv

	case GreaterThanOrEqualTo:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv >= tv

	case LessThan:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv < tv

	case LessThanOrEqualTo:
		sv, tv, ok := parseNum(stored, target)
		if !ok {
			return false
		}
		return sv <= tv

	case NotEqualTo:
		return stored != target

	case Like:
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
