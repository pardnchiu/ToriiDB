package daemon

import (
	"context"
	"encoding/json"
	"fmt"
)

func (d *Daemon) Get(ctx context.Context, db int, key string) (*Record, error) {
	res, err := d.call(ctx, request{Op: opGet, DB: db, Key: key})
	if err != nil {
		return nil, err
	}
	return res.Record, nil
}

func (d *Daemon) Entries(ctx context.Context, db int, pattern string, option *ScanOption) ([]Record, error) {
	req := request{Op: opEntries, DB: db, Pattern: pattern}
	if option != nil {
		req.Contains = option.Contains
		req.After = option.After
		req.Limit = option.Limit
	}

	res, err := d.call(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Entries, nil
}

func (d *Daemon) Set(ctx context.Context, db int, key string, content any, expireAt *int64) error {
	return d.set(ctx, db, key, content, expireAt, false)
}

func (d *Daemon) SetVector(ctx context.Context, db int, key string, content any, expireAt *int64) error {
	return d.set(ctx, db, key, content, expireAt, true)
}

func (d *Daemon) set(ctx context.Context, db int, key string, content any, expireAt *int64, vector bool) error {
	raw, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode content: %w", err)
	}

	_, err = d.call(ctx, request{
		Op:       opSet,
		DB:       db,
		Key:      key,
		Content:  raw,
		ExpireAt: expireAt,
		Vector:   vector,
	})
	return err
}

func (d *Daemon) Del(ctx context.Context, db int, keys ...string) (int, error) {
	return d.count(ctx, request{Op: opDel, DB: db, Keys: keys})
}

func (d *Daemon) TTL(ctx context.Context, db int, key string) (int64, error) {
	res, err := d.call(ctx, request{Op: opTTL, DB: db, Key: key})
	if err != nil {
		return 0, err
	}
	return *res.TTL, nil
}

func (d *Daemon) Expire(ctx context.Context, db int, ttl int64, keys ...string) (int, error) {
	return d.count(ctx, request{Op: opExpire, DB: db, Keys: keys, TTL: &ttl})
}

func (d *Daemon) count(ctx context.Context, req request) (int, error) {
	if len(req.Keys) == 0 {
		return 0, nil
	}

	res, err := d.call(ctx, req)
	if err != nil {
		return 0, err
	}
	return *res.Count, nil
}

func (d *Daemon) Keys(ctx context.Context, db int, pattern string, limit, page int) (*ListResult, error) {
	res, err := d.call(ctx, request{Op: opKeys, DB: db, Pattern: pattern, Limit: limit, Page: page})
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

func (d *Daemon) VSearch(ctx context.Context, db int, text, pattern string, limit int) ([]string, error) {
	res, err := d.call(ctx, request{Op: opVSearch, DB: db, Text: text, Pattern: pattern, Limit: limit})
	if err != nil {
		return nil, err
	}
	return res.Keys, nil
}

func (d *Daemon) HasEmbedder(ctx context.Context) (bool, error) {
	res, err := d.call(ctx, request{Op: opEmbedder})
	if err != nil {
		return false, err
	}
	return res.Embedder != nil && *res.Embedder, nil
}

func (d *Daemon) call(ctx context.Context, req request) (*response, error) {
	if d.mode == ModeClient {
		return d.client.call(ctx, req)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", req.Op, err)
	}

	res := d.dispatch(ctx, &req)
	if res.Error != "" {
		return nil, remoteError(res.Kind, res.Error)
	}
	return &res, nil
}
