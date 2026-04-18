package store

import (
	"errors"
	"fmt"
)

var (
	errVectorMissing  = errors.New("vector is missing")
	errVectorMismatch = errors.New("vector is mismatch")
)

func (c *core) VSim(key1, key2 string) (float64, error) {
	e1, ok := c.Get(key1)
	if !ok {
		return 0, errVectorMissing
	}

	e2, ok := c.Get(key2)
	if !ok {
		return 0, errVectorMissing
	}

	if len(e1.Vector) == 0 || len(e2.Vector) == 0 {
		return 0, errVectorMissing
	}

	if len(e1.Vector) != len(e2.Vector) {
		return 0, fmt.Errorf("%w: %d vs %d", errVectorMismatch, len(e1.Vector), len(e2.Vector))
	}

	score, ok := cosine(e1.Vector, e2.Vector)
	if !ok {
		return 0, errVectorMissing
	}
	return score, nil
}

func (c *core) VGet(key string) ([]float32, bool) {
	e, ok := c.Get(key)
	if !ok || len(e.Vector) == 0 {
		return nil, false
	}
	out := make([]float32, len(e.Vector))
	copy(out, e.Vector)
	return out, true
}
