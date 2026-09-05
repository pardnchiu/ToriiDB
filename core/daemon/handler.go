package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/toriidb/core/store"
)

const defaultLimit = 100

func (d *Daemon) dispatch(ctx context.Context, req *request) response {
	session := d.torii.Session()
	if err := session.Select(req.DB); err != nil {
		return failure(kindBadRequest, err.Error())
	}

	switch req.Op {
	case opGet:
		return handleGet(session, req)
	case opEntries:
		return handleEntries(session, req)
	case opSet:
		return handleSet(ctx, session, req)
	case opDel:
		return handleDel(session, req)
	case opTTL:
		return handleTTL(session, req)
	case opExpire:
		return handleExpire(session, req)
	case opKeys:
		return handleKeys(session, req)
	case opVSearch:
		return handleVSearch(ctx, session, req)
	}

	return failure(kindBadRequest, fmt.Sprintf("unknown op %q", req.Op))
}

func toRecord(db int, key string, entry *store.Entry) (Record, error) {
	content, err := json.Marshal(decodeContent(entry.Type, entry.Value()))
	if err != nil {
		return Record{}, err
	}

	return Record{
		DB:        db,
		Key:       key,
		Type:      entry.Type.String(),
		Content:   content,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
		ExpireAt:  entry.ExpireAt,
	}, nil
}

func handleGet(session *store.Session, req *request) response {
	entry, found := session.Get(req.Key)
	if !found {
		return failure(kindNotFound, "key not found")
	}

	record, err := toRecord(session.Current(), req.Key, entry)
	if err != nil {
		return failure(kindInternal, err.Error())
	}

	return response{Record: &record}
}

func handleEntries(session *store.Session, req *request) response {
	pattern := req.Pattern
	if pattern == "" {
		return failure(kindBadRequest, "pattern is required")
	}

	contains := strings.ToLower(req.Contains)
	db := session.Current()
	keys := session.Keys(pattern)
	list := make([]Record, 0, len(keys))

	for _, one := range keys {
		entry, found := session.Get(one)
		if !found {
			continue
		}
		if req.After > 0 && entryUnix(entry) < req.After {
			continue
		}
		if contains != "" && !strings.Contains(strings.ToLower(entry.Value()), contains) {
			continue
		}

		record, err := toRecord(db, one, entry)
		if err != nil {
			return failure(kindInternal, err.Error())
		}
		list = append(list, record)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if len(list) > limit {
		list = list[len(list)-limit:]
	}

	return response{Entries: list}
}

func entryUnix(entry *store.Entry) int64 {
	if entry.UpdatedAt != nil {
		return *entry.UpdatedAt
	}
	return entry.CreatedAt
}

func handleSet(ctx context.Context, session *store.Session, req *request) response {
	value, err := encodeContent(req.Content)
	if err != nil {
		return failure(kindBadRequest, err.Error())
	}

	if req.Vector {
		err = session.SetVector(ctx, req.Key, value, store.SetDefault, req.ExpireAt)
	} else {
		err = session.Set(req.Key, value, store.SetDefault, req.ExpireAt)
	}
	if err != nil {
		return failure(kindInternal, err.Error())
	}

	return response{}
}

func handleDel(session *store.Session, req *request) response {
	if len(req.Keys) == 0 {
		return failure(kindBadRequest, "keys is required")
	}

	count := session.Del(req.Keys...)
	return response{Count: &count}
}

func handleTTL(session *store.Session, req *request) response {
	ttl := session.TTL(req.Key)
	if ttl == -2 {
		return failure(kindNotFound, "key not found")
	}

	return response{TTL: &ttl}
}

func handleExpire(session *store.Session, req *request) response {
	if len(req.Keys) == 0 {
		return failure(kindBadRequest, "keys is required")
	}
	if req.TTL == nil {
		return failure(kindBadRequest, "ttl is required")
	}
	if *req.TTL <= 0 {
		return failure(kindBadRequest, "ttl must be a positive integer")
	}

	count := 0
	for _, one := range req.Keys {
		if err := session.Expire(one, *req.TTL); err == nil {
			count++
		}
	}

	return response{Count: &count}
}

func handleKeys(session *store.Session, req *request) response {
	pattern := req.Pattern
	if pattern == "" {
		return failure(kindBadRequest, "pattern is required")
	}

	keys := session.Keys(pattern)
	total := len(keys)
	limit, page, from, to := 0, 1, 0, total

	if req.Limit > 0 {
		limit = req.Limit
		page = max(req.Page, 1)
		from = min((page-1)*limit, total)
		to = min(from+limit, total)
	}

	return response{List: &ListResult{
		DB:    session.Current(),
		Keys:  keys[from:to],
		Total: total,
		Limit: limit,
		Page:  page,
	}}
}

func handleVSearch(ctx context.Context, session *store.Session, req *request) response {
	pattern := req.Pattern
	if pattern == "" {
		return failure(kindBadRequest, "pattern is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return failure(kindBadRequest, "text is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	keys, err := session.VSearch(ctx, req.Text, pattern, limit)
	if err != nil {
		return failure(kindInternal, err.Error())
	}

	return response{Keys: keys}
}
