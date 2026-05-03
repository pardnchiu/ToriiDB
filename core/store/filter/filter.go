package filter

import (
	"fmt"
	"strings"

	"github.com/agenvoy/toriidb/core/utils"
)

type Filter interface {
	Match(obj any) bool
}

type Cond struct {
	Field string
	Op    Operator
	Value string
}

func (c Cond) Match(obj any) bool {
	val, ok := getValue(c.Field, obj)
	return ok && Match(val, c.Op, c.Value)
}

func getValue(key string, obj any) (string, bool) {
	var subKeys []string
	for part := range strings.SplitSeq(key, ".") {
		if part != "" {
			subKeys = append(subKeys, part)
		}
	}

	val, ok := utils.WalkKeys(obj, subKeys)
	if !ok {
		return "", false
	}
	return utils.Vtoa(val), true
}

func AtoFilter(str string) (Filter, error) {
	var words []string
	for word := range strings.FieldsSeq(str) {
		// * /\(*/
		for strings.HasPrefix(word, "(") {
			words = append(words, "(")
			word = word[1:]
		}

		// * /\)*/
		var trailing int
		for strings.HasSuffix(word, ")") {
			trailing++
			word = word[:len(word)-1]
		}

		if word != "" {
			words = append(words, word)
		}

		for range trailing {
			words = append(words, ")")
		}
	}

	if len(words) == 0 {
		return nil, fmt.Errorf("no filter expression")
	}

	parser := &Parser{Words: words}
	filter, err := parser.Or()
	if err != nil {
		return nil, err
	}

	// * parse error
	if parser.Position < len(parser.Words) {
		return nil, fmt.Errorf("unexpected word: %s", parser.Current())
	}
	return filter, nil
}
