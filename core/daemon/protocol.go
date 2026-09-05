package daemon

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	headerSize   = 4
	maxFrameSize = 8 << 20
)

const (
	opGet     = "get"
	opEntries = "entries"
	opSet     = "set"
	opDel     = "del"
	opTTL     = "ttl"
	opExpire  = "expire"
	opKeys    = "keys"
	opVSearch = "vsearch"
)

const (
	kindNotFound   = "not_found"
	kindBadRequest = "bad_request"
	kindInternal   = "internal"
)

type request struct {
	Op       string          `json:"op"`
	DB       int             `json:"db"`
	Key      string          `json:"key,omitempty"`
	Keys     []string        `json:"keys,omitempty"`
	Pattern  string          `json:"pattern,omitempty"`
	Text     string          `json:"text,omitempty"`
	Contains string          `json:"contains,omitempty"`
	After    int64           `json:"after,omitempty"`
	Content  json.RawMessage `json:"content,omitempty"`
	Vector   bool            `json:"vector,omitempty"`
	ExpireAt *int64          `json:"expire_at,omitempty"`
	TTL      *int64          `json:"ttl,omitempty"`
	Limit    int             `json:"limit,omitempty"`
	Page     int             `json:"page,omitempty"`
}

type ScanOption struct {
	Contains string
	After    int64
	Limit    int
}

type Record struct {
	DB        int             `json:"db"`
	Key       string          `json:"key"`
	Type      string          `json:"type"`
	Content   json.RawMessage `json:"content,omitempty"`
	CreatedAt int64           `json:"created_at,omitempty"`
	UpdatedAt *int64          `json:"updated_at,omitempty"`
	ExpireAt  *int64          `json:"expire_at,omitempty"`
}

type ListResult struct {
	DB    int      `json:"db"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
	Total int      `json:"total"`
	Keys  []string `json:"keys"`
}

type response struct {
	Error   string      `json:"error,omitempty"`
	Kind    string      `json:"kind,omitempty"`
	Record  *Record     `json:"record,omitempty"`
	Entries []Record    `json:"entries,omitempty"`
	List    *ListResult `json:"list,omitempty"`
	Keys    []string    `json:"keys,omitempty"`
	TTL     *int64      `json:"ttl,omitempty"`
	Count   *int        `json:"count,omitempty"`
}

func failure(kind, message string) response {
	return response{Error: message, Kind: kind}
}

func remoteError(kind, message string) error {
	switch kind {
	case kindNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, message)
	case kindBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, message)
	}
	return errors.New(message)
}

func writeFrame(w io.Writer, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode frame: %w", err)
	}

	buf := make([]byte, headerSize+len(raw))
	binary.BigEndian.PutUint32(buf, uint32(len(raw)))
	copy(buf[headerSize:], raw)

	_, err = w.Write(buf)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	var head [headerSize]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(head[:])
	if size > maxFrameSize {
		return nil, fmt.Errorf("frame is %d bytes, limit is %d", size, maxFrameSize)
	}

	raw := make([]byte, size)
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, err
	}

	return raw, nil
}
