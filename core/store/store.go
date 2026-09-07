package store

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	"github.com/pardnchiu/toriidb/core/openai"
)

const (
	defaultDir = "./temp"
	maxDB      = 16
)

type db struct {
	mu       sync.RWMutex
	dir      string
	data     map[string]*Entry
	aof      *os.File
	num      int
	loadErr  error
	logSize  int64
	snapSize int64
	once     sync.Once
	loaded   bool
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

func (c *core) HasEmbedder() bool {
	return c.embedder != nil
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
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

type AOFRecord struct {
	Timestamp int64   `json:"ts"`
	Command   string  `json:"cmd"`
	Key       string  `json:"key"`
	Value     string  `json:"value,omitempty"`
	ExpireAt  *int64  `json:"expire_at,omitempty"`
	Vector    *string `json:"vector,omitempty"`
}

func New(dir string, apiKey ...string) (*Store, error) {
	key := ""
	switch len(apiKey) {
	case 0:
	case 1:
		key = apiKey[0]
	default:
		return nil, fmt.Errorf("just one api key")
	}

	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return nil, err
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
	if client, err := openai.New(key); err == nil {
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
		d.loadAll()
		d.loaded = true
	})
}

func (d *db) loadAll() {
	stale, _ := filepath.Glob(filepath.Join(d.dir, "*.tmp"))
	for _, path := range stale {
		slog.Warn("toriidb: removing stale temp file", slog.String("file", path))
		go_pkg_filesystem.Remove(path)
	}

	nums := snapNumbers(d.dir)
	if len(nums) == 0 {
		nums = []int{0}
	}

	for _, n := range nums {
		if d.replayGeneration(n) {
			return
		}
	}
}

// * n = 0 代表沒有 snapshot（全新 db），只重播 log
func (d *db) replayGeneration(n int) bool {
	data := make(map[string]*Entry)

	if n > 0 {
		snap := snapPath(d.dir, n)
		if err := replayInto(data, snap); err != nil {
			d.loadErr = fmt.Errorf("replay %s: %w", snap, err)
			return false
		}
		d.snapSize = sizeOf(snap)
	}

	logSize := int64(0)
	num := n + 1
	for {
		log := logPath(d.dir, num)
		if !go_pkg_filesystem_reader.Exists(log) {
			break
		}
		if err := replayInto(data, log); err != nil {
			d.loadErr = fmt.Errorf("replay %s: %w", log, err)
			return false
		}
		logSize = sizeOf(log)
		num++
	}

	d.loadErr = nil
	d.data = data
	d.logSize = logSize
	d.num = max(num-1, n+1)
	return true
}

func (d *db) init() error {
	if d.aof != nil {
		return nil
	}

	if err := go_pkg_filesystem.CheckDir(d.dir, true); err != nil {
		return err
	}

	file, err := os.OpenFile(logPath(d.dir, d.num), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}

	d.aof = file
	return nil
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() { s.closeErr = s.closeAll() })
	return s.closeErr
}

func (s *Store) closeAll() error {
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
			} else {
				gcOlderThan(d.dir, d.num-1)
			}
			if d.aof != nil {
				d.aof.Close()
				d.aof = nil
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
		}
	}
}
