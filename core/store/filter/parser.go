package filter

import (
	"fmt"
	"strings"
)

type Parser struct {
	Words    []string
	Position int
}

func (p *Parser) Current() string {
	if p.Position >= len(p.Words) {
		return ""
	}
	return p.Words[p.Position]
}

func (p *Parser) next() string {
	word := p.Current()
	if word != "" {
		p.Position++
	}
	return word
}

func (p *Parser) Primary() (Filter, error) {
	if p.Current() == "(" {
		p.next()
		f, err := p.Or()
		if err != nil {
			return nil, err
		}
		if p.next() != ")" {
			return nil, fmt.Errorf("')' is required")
		}
		return f, nil
	}

	word := p.next()
	if word == "" {
		return nil, fmt.Errorf("word is required")
	}

	operator := p.next()
	if operator == "" {
		return nil, fmt.Errorf("operator is required after %s", word)
	}

	op, ok := AtoOperation(operator)
	if !ok {
		return nil, fmt.Errorf("invalid operator: %s", operator)
	}

	value := p.next()
	if value == "" {
		return nil, fmt.Errorf("value is required after %s %s", word, operator)
	}
	return Cond{Field: word, Op: op, Value: value}, nil
}

func (p *Parser) Or() (Filter, error) {
	left, err := p.And()
	if err != nil {
		return nil, err
	}

	filters := []Filter{left}
	for strings.ToUpper(p.Current()) == "OR" {
		p.next()
		right, err := p.And()
		if err != nil {
			return nil, err
		}
		filters = append(filters, right)
	}

	if len(filters) == 1 {
		return filters[0], nil
	}
	return Or(filters), nil
}

func (p *Parser) And() (Filter, error) {
	left, err := p.Not()
	if err != nil {
		return nil, err
	}

	filters := []Filter{left}
	for strings.ToUpper(p.Current()) == "AND" {
		p.next()
		right, err := p.Not()
		if err != nil {
			return nil, err
		}
		filters = append(filters, right)
	}

	if len(filters) == 1 {
		return filters[0], nil
	}
	return And(filters), nil
}

func (p *Parser) Not() (Filter, error) {
	if strings.ToUpper(p.Current()) == "NOT" {
		p.next()
		f, err := p.Not()
		if err != nil {
			return nil, err
		}
		return Not{f}, nil
	}
	return p.Primary()
}
