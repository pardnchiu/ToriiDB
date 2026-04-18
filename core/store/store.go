package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pardnchiu/ToriiDB/core/openai"
)

const (
	defaultDir = "./temp"
	maxDB      = 16
)

type db struct {
	mu              sync.RWMutex
	dir             string
	data            map[string]*Entry
	aof             *os.File
	aofSize         int64
	aofSizeBaseline int64
	once            sync.Once
	loaded          bool
}

type embedder struct {
	embed func(ctx context.Context, text string) ([]float32, error)
	dim   int
	model string
}

type Session struct {
	core
}

type core struct {
	dbs      *[maxDB]*db
	db       int
	embedder *embedder
	wg       *sync.WaitGroup
}

func (c *core) DB() *db {
	d := c.dbs[c.db]
	d.ensureLoaded()
	return d
}

func (c *core) Current() int {
	return c.db
}

func (c *core) Select(index int) error {
	if index < 0 || index >= maxDB {
		return fmt.Errorf("invalid db index: %d (0-%d)", index, maxDB-1)
	}
	c.db = index
	return nil
}

type Store struct {
	allDBs [maxDB]*db
	core
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type AOFRecord struct {
	Timestamp int64   `json:"ts"`
	Command   string  `json:"cmd"`
	Key       string  `json:"key"`
	Value     string  `json:"value,omitempty"`
	ExpireAt  *int64  `json:"expire_at,omitempty"`
	Vector    *string `json:"vector,omitempty"`
}

func New(path ...string) (*Store, error) {
	dir := defaultDir

	switch len(path) {
	case 0:
	case 1:
		dir = path[0]
	default:
		return nil, fmt.Errorf("just one path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("os.Stat %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("os.MkdirAll %s: %w", dir, err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("not directory")
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Store{
		cancel: cancel,
	}

	for i := range maxDB {
		s.allDBs[i] = &db{
			dir:  filepath.Join(dir, fmt.Sprintf("db_%d", i)),
			data: make(map[string]*Entry),
		}
	}

	s.core.dbs = &s.allDBs
	s.core.wg = &s.wg
	if client, err := openai.New(); err == nil {
		s.core.embedder = &embedder{
			embed: client.Embed,
			dim:   client.Dim,
			model: client.Model,
		}
	}

	go s.cleanTimer(ctx, time.Minute)

	return s, nil
}

func (d *db) ensureLoaded() {
	d.once.Do(func() {
		aofPath := filepath.Join(d.dir, "record.aof")
		if data, size, err := replayAOF(aofPath); err == nil {
			d.data = data
			d.aofSize = size
			d.aofSizeBaseline = size
		}
		d.loaded = true
	})
}

func (d *db) init() error {
	if d.aof != nil {
		return nil
	}

	if err := os.MkdirAll(d.dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	aofPath := filepath.Join(d.dir, "record.aof")
	file, err := os.OpenFile(aofPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open aof: %w", err)
	}

	d.aof = file
	return nil
}

func (s *Store) Close() error {
	s.wg.Wait()
	s.cancel()

	errs := make(chan error, maxDB)
	var wg sync.WaitGroup

	for _, d := range s.allDBs {
		if !d.loaded {
			continue
		}

		wg.Add(1)
		go func(d *db) {
			defer wg.Done()
			d.mu.Lock()
			defer d.mu.Unlock()
			if err := d.compact(); err != nil {
				errs <- err
			}
		}(d)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}

	return nil
}

func (s *Store) Session() *Session {
	return &Session{core: core{
		dbs:      &s.allDBs,
		db:       s.core.db,
		embedder: s.core.embedder,
		wg:       s.core.wg,
	}}
}

func (s *Store) cleanTimer(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for _, d := range s.allDBs {
				if !d.loaded {
					continue
				}
				d.cleanExpired()
			}
			timer.Reset(interval)
		}
	}
}

func (d *db) cleanExpired() {
	now := time.Now().Unix()

	d.mu.Lock()
	defer d.mu.Unlock()

	for key, e := range d.data {
		if e.ExpireAt != nil && *e.ExpireAt <= now {
			delete(d.data, key)
			os.Remove(d.filePath(key))
		}
	}
}
