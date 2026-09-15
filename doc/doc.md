# toriidb - Documentation

Last updated: 2026-09-16

> Back to [README](../README.md)

## Prerequisites

- Go 1.25 or higher
- A Unix-like system with unix domain sockets (the daemon listens under `/tmp`)
- A writable data directory
- Optional: `OPENAI_API_KEY`, needed only for vector operations

## Installation

### Using go get

```bash
go get github.com/pardnchiu/toriidb
```

### From Source

```bash
git clone https://github.com/pardnchiu/toriidb.git
cd toriidb
go build ./...
```

### Make Targets

| Target | Action |
|---|---|
| `make test` | Start the REPL on `./temp` |
| `make unit` | `go test ./core/... -count=1 -cover` |
| `make embed` | Run the OpenAI embedding integration test |

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `OPENAI_API_KEY` | No | none | Enables embeddings; without it vector operations return `ErrNoEmbedder` |
| `TORIIDB_EMBED_DIM` | No | `256` | Embedding dimension; values `<= 0` fall back to `256` |

### API Key Lookup Order

1. The optional `apiKey` argument of `store.New(dir, apiKey)` / `daemon.New(dir, apiKey)`
2. The `OPENAI_API_KEY` environment variable
3. The OS keychain entry `OPENAI_API_KEY` under service `ToriiDB`

### Config File

The Go code never loads `.env`. The `Makefile` `include`s and `export`s a local `.env` when present:

```bash
cat <<'EOF' > .env
OPENAI_API_KEY=your_api_key
TORIIDB_EMBED_DIM=256
EOF
make test
```

## Usage

### Basic

Open a data directory through the socket daemon. The first caller on a directory becomes the server and every later caller becomes a client, using the same methods. `Start` does nothing in client mode:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/pardnchiu/toriidb/core/daemon"
)

func main() {
	d, err := daemon.New("./data")
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	if err := d.Start(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	if err := d.Set(ctx, 0, "user:1", map[string]any{"name": "Torii", "level": 1}, nil); err != nil {
		log.Fatal(err)
	}

	record, err := d.Get(ctx, 0, "user:1")
	if errors.Is(err, daemon.ErrNotFound) {
		fmt.Println("missing")
		return
	}
	if err != nil {
		log.Fatal(err)
	}

	value, err := record.Value()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(record.Type, value)
}
```

When one process owns the directory, use the embedded store directly:

```go
package main

import (
	"fmt"
	"log"

	"github.com/pardnchiu/toriidb/core/store"
)

func main() {
	torii, err := store.New("./data")
	if err != nil {
		log.Fatal(err)
	}
	defer torii.Close()

	if err := torii.Set("counter", "1", store.SetDefault, nil); err != nil {
		log.Fatal(err)
	}

	n, err := torii.Incr("counter", 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(n)
	fmt.Println(torii.Exec("GET counter"))
}
```

REPL:

```bash
go run cmd/test/main.go
```

```text
toriidb[0]> SET user {"name":"Torii","profile":{"level":1}}
OK
toriidb[0]> GET user.profile.level
1
toriidb[0]> INCR user.profile.level
2
toriidb[0]> SET session abc123 300
OK
toriidb[0]> TTL session
(integer) 300
```

### Advanced

JSON fields and query expressions. `SetField` updates one nested field without rewriting the document, and each `Query` row is `key: rawValue`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/pardnchiu/toriidb/core/store"
	"github.com/pardnchiu/toriidb/core/store/filter"
)

func main() {
	torii, err := store.New("./data")
	if err != nil {
		log.Fatal(err)
	}
	defer torii.Close()

	if err := torii.Set("user:1", `{"role":"admin","profile":{"level":3}}`, store.SetDefault, nil); err != nil {
		log.Fatal(err)
	}

	if err := torii.SetField("user:1", []string{"profile", "level"}, "4", store.SetDefault, nil); err != nil {
		log.Fatal(err)
	}

	f, err := filter.AtoFilter("role = admin AND profile.level >= 3")
	if err != nil {
		log.Fatal(err)
	}

	for _, row := range torii.Query(f, 10) {
		fmt.Println(row)
	}
}
```

Vector search, paging, and server-side filtering through the daemon. Vectors attach in the background, so a freshly written key appears in `VSearch` results shortly after `SetVector` returns:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/pardnchiu/toriidb/core/daemon"
)

func main() {
	d, err := daemon.New("./data")
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	if err := d.Start(); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	expireAt := time.Now().Add(time.Hour).Unix()
	err = d.SetVector(ctx, 0, "article:1", "embedded JSON database", &expireAt)
	if errors.Is(err, daemon.ErrNoEmbedder) {
		log.Fatal("OPENAI_API_KEY is not configured")
	}
	if err != nil {
		log.Fatal(err)
	}

	keys, err := d.VSearch(ctx, 0, "local semantic storage", "article:*", 5)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(keys)

	page, err := d.Keys(ctx, 0, "article:*", 20, 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(page.Total, page.Keys)

	records, err := d.Entries(ctx, 0, "article:*", &daemon.ScanOption{Contains: "json", Limit: 50})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(records))

	removed, err := d.Del(ctx, 0, page.Keys...)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(removed)
}
```

## API Reference

### daemon

| Function / Method | Description |
|---|---|
| `New(dir string, apiKey ...string) (*Daemon, error)` | Resolves the directory and derives the socket; a live socket means `ModeClient`, otherwise it opens the store as `ModeServer` |
| `(*Daemon) Start() error` | Starts listening without blocking; returns `ErrAlreadyRunning` if another daemon serves the socket; no-op in client mode |
| `(*Daemon) Stop() error` | Closes the listener and every connection, then waits for handler goroutines |
| `(*Daemon) Close() error` | Server: `Stop` then close the store; client: close idle connections |
| `(*Daemon) Mode() Mode` / `Socket() string` / `Store() *store.Store` | Mode, socket path, underlying store (`nil` in client mode) |
| `Get(ctx, db, key) (*Record, error)` | Reads one record; `ErrNotFound` when missing |
| `Entries(ctx, db, pattern, *ScanOption) ([]Record, error)` | Server-side filtering by `Contains`/`After`/`Limit` (default 100, keeps the last n) |
| `Set(ctx, db, key, content any, expireAt *int64) error` | Upsert; strings are stored as text, other values as compact JSON |
| `SetVector(ctx, db, key, content any, expireAt *int64) error` | Upsert plus background embedding |
| `Del(ctx, db, keys ...string) (int, error)` | Returns the number removed |
| `TTL(ctx, db, key) (int64, error)` | Remaining seconds, `-1` when persistent |
| `Expire(ctx, db, ttl int64, keys ...string) (int, error)` | Returns keys that received the TTL; `ttl` must be `> 0` |
| `Keys(ctx, db, pattern, limit, page) (*ListResult, error)` | `limit <= 0` returns every key |
| `VSearch(ctx, db, text, pattern, limit) ([]string, error)` | Ordered by similarity; `limit <= 0` means `100` |
| `HasEmbedder(ctx) (bool, error)` | Whether the server has an embedder |
| `(*Record) Value() (string, error)` | Returns the stored text; do not use `string(record.Content)` |

Errors: `ErrAlreadyRunning`, `ErrNotFound`, `ErrBadRequest`, `ErrNoEmbedder`; check with `errors.Is`.

### store

`Store` and `Session` share one method set; concurrent callers each use their own `Store.Session()`.

| Function / Method | Description |
|---|---|
| `New(dir string, apiKey ...string) (*Store, error)` | `dir` is required; the 16 databases load on first use |
| `(*Store) Session() *Session` | Shares data, holds its own selected database index |
| `(*Store) Close() error` | Idempotent; waits for vector jobs, compacts, and removes old generations |
| `Select(index int) error` / `Current() int` | Switch / read the database (`0-15`) |
| `Exec(input string) string` | Runs REPL command text |
| `Set` / `SetField` / `SetVector` | Write a value, a nested field, or a value with a vector; `SetFlag` is `SetDefault`/`SetNX`/`SetXX` |
| `Get(key) (*Entry, bool)` / `GetField(key, subKeys) (string, bool)` | Read an entry copy or a nested field |
| `Incr(key, delta)` / `IncrField(key, subKeys, delta)` | Numeric increment |
| `Del(keys...) int` / `DelField(key, subKeys) error` | Delete keys or a field |
| `TTL` / `Expire` / `ExpireAt` / `Persist` | Expiry management; `TTL` is `-2` when missing |
| `Keys(pattern) []string` | `path.Match` glob, lexicographic |
| `Find(op, value, limit) []string` | Compares whole raw values, newest first |
| `Query(f filter.Filter, limit) []string` | Filters JSON documents, returns `key: value` |
| `VSearch(ctx, text, pattern, k)` / `VSim(key1, key2)` / `VGet(key)` | Vector search, similarity, vector retrieval; `k <= 0` means `10` |
| `HasEmbedder() bool` | Whether an embedder was created |

### filter

| Name | Description |
|---|---|
| `AtoFilter(str string) (Filter, error)` | Parses `field op value`, `AND`/`OR`/`NOT`, and parentheses |
| `AtoOperation(s string) (Operator, bool)` | `EQ`/`=`, `NE`/`!=`, `GT`/`>`, `GTE`/`GE`/`>=`, `LT`/`<`, `LTE`/`LE`/`<=`, `LIKE` |
| `And` / `Or` / `Not` / `Cond` / `EQ` ... `LIKE` | Composable programmatic filter types |

### REPL Commands

| Command | Syntax |
|---|---|
| `GET` / `EXIST` / `TYPE` | `GET <key>` (`key.field` reads a nested field) |
| `SET` | `SET <key> <value> [NX\|XX] [seconds] [VECTOR]` |
| `DEL` | `DEL <key> [key2] ...` |
| `INCR` | `INCR <key> [delta]` |
| `TTL` / `EXPIRE` / `EXPIREAT` / `PERSIST` | `EXPIRE <key> <seconds>`, `EXPIREAT <key> <timestamp>` |
| `KEYS` | `KEYS <pattern>` |
| `FIND` | `FIND <op> <value> [LIMIT <n>]` |
| `QUERY` | `QUERY <expression> [LIMIT <n>]` |
| `VSEARCH` | `VSEARCH <text> [MATCH <pattern>] [LIMIT <n>]` |
| `VSIM` / `VGET` | `VSIM <key1> <key2>`, `VGET <key>` |
| `SELECT` | `SELECT <db>` (`0-15`) |

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
