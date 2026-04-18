package store

import (
	"container/heap"
	"context"
	"fmt"
	"path"
	"strings"
	"time"
)

const defaultVSearchLimit = 10

type vsearchItem struct {
	key   string
	score float64
}

type vsearchMinHeap []vsearchItem

func (h vsearchMinHeap) Len() int           { return len(h) }
func (h vsearchMinHeap) Less(i, j int) bool { return h[i].score < h[j].score }
func (h vsearchMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *vsearchMinHeap) Push(x any) {
	*h = append(*h, x.(vsearchItem))
}

func (h *vsearchMinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (c *core) VSearch(ctx context.Context, text, pattern string, k int) ([]string, error) {
	if c.embedder == nil {
		return nil, errEmbedderNotConfigured
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("vsearch text is empty")
	}
	if k <= 0 {
		k = defaultVSearchLimit
	}

	qVec, err := c.resolveQueryVector(ctx, text)
	if err != nil {
		return nil, err
	}

	d := c.DB()

	d.mu.RLock()
	defer d.mu.RUnlock()

	return scanTopK(d, qVec, pattern, k, time.Now().Unix()), nil
}

func scanTopK(d *db, qVec []float32, pattern string, k int, now int64) []string {
	h := &vsearchMinHeap{}

	for key, e := range d.data {
		if isInternal(key) {
			continue
		}
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			continue
		}
		if len(e.Vector) == 0 || len(e.Vector) != len(qVec) {
			continue
		}
		if pattern != "" {
			matched, err := path.Match(pattern, key)
			if err != nil || !matched {
				continue
			}
		}

		score, ok := cosine(e.Vector, qVec)
		if !ok {
			continue
		}

		if h.Len() < k {
			heap.Push(h, vsearchItem{key: key, score: score})
			continue
		}
		if score > (*h)[0].score {
			(*h)[0] = vsearchItem{key: key, score: score}
			heap.Fix(h, 0)
		}
	}

	out := make([]vsearchItem, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(vsearchItem)
	}

	result := make([]string, len(out))
	for i, item := range out {
		result[i] = item.key
	}
	return result
}

func (c *core) resolveQueryVector(ctx context.Context, text string) ([]float32, error) {
	d := c.DB()
	model := c.embedder.model
	dim := c.embedder.dim

	d.mu.RLock()
	if vec, ok := d.getVector(model, dim, text); ok {
		d.mu.RUnlock()
		return vec, nil
	}
	d.mu.RUnlock()

	vec, err := c.embedder.embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	d.mu.Lock()
	_ = d.putVector(model, dim, text, vec)
	d.mu.Unlock()

	return vec, nil
}
