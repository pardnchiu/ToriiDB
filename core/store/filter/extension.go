package filter

type And []Filter

type Or []Filter

type Not [1]Filter

func (a And) Match(obj any) bool {
	for _, f := range a {
		if !f.Match(obj) {
			return false
		}
	}
	return true
}

func (o Or) Match(obj any) bool {
	for _, f := range o {
		if f.Match(obj) {
			return true
		}
	}
	return false
}

func (n Not) Match(obj any) bool {
	return !n[0].Match(obj)
}

type EQ struct {
	Field string
	Value string
}

type NE struct {
	Field string
	Value string
}

type GT struct {
	Field string
	Value string
}

type GTE struct {
	Field string
	Value string
}

type GE struct {
	Field string
	Value string
}

type LT struct {
	Field string
	Value string
}

type LTE struct {
	Field string
	Value string
}

type LE struct {
	Field string
	Value string
}

type LIKE struct {
	Field string
	Value string
}

func (c EQ) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, EqualTo, c.Value)
}

func (c NE) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, NotEqualTo, c.Value)
}

func (c GT) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, GreaterThan, c.Value)
}

func (c GTE) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, GreaterThanOrEqualTo, c.Value)
}

func (c GE) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, GreaterThanOrEqualTo, c.Value)
}

func (c LT) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, LessThan, c.Value)
}
func (c LTE) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, LessThanOrEqualTo, c.Value)
}

func (c LE) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, LessThanOrEqualTo, c.Value)
}

func (c LIKE) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, Like, c.Value)
}
