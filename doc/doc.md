# ToriiDB - Documentation

> Back to [README](../README.md)

## Prerequisites

- Go 1.25 or higher
- A writable local directory (defaults to `./temp`)
- **Optional**: `OPENAI_API_KEY` in `.env` (required only for `SET ... VECTOR` / `VSEARCH` / `VSIM` / `VGET`)

Core storage has no external service dependency. Vector features lazily initialize the OpenAI client on first use and no-op if the key is absent.

## Installation

### Using go get

```bash
go get github.com/pardnchiu/ToriiDB
```

### .env for vector features

```bash
# .env
OPENAI_API_KEY=sk-...
```

### Build the REPL from source

```bash
git clone https://github.com/pardnchiu/ToriiDB.git
cd ToriiDB
go build -o toriidb ./cmd/test
./toriidb
```

### Run the REPL directly

```bash
go run ./cmd/test
```

## Usage

### Initialize a Store

```go
package main

import (
    "fmt"
    "log"

    "github.com/pardnchiu/ToriiDB/core/store"
)

func main() {
    s, err := store.New()
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    if err := s.Set("user:1", `{"name":"Alice","age":25}`, store.SetDefault, nil); err != nil {
        log.Fatal(err)
    }

    entry, ok := s.Get("user:1")
    if !ok {
        log.Fatal("not found")
    }
    fmt.Println(entry.Value())
}
```

### Custom storage path and Sessions

```go
s, err := store.New("/data/torii")
if err != nil {
    log.Fatal(err)
}
defer s.Close()

if err := s.Select(6); err != nil {
    log.Fatal(err)
}

sess := s.Session()
if err := sess.Select(3); err != nil {
    log.Fatal(err)
}
// s.Current() == 6, sess.Current() == 3
```

### TTL and expiration

```go
exp := time.Now().Unix() + 60
if err := s.Set("token:abc", "payload", store.SetDefault, &exp); err != nil {
    log.Fatal(err)
}

ttl := s.TTL("token:abc")          // remaining seconds
_ = s.Expire("token:abc", 300)      // expire in 300s
_ = s.Persist("token:abc")          // remove expiration
```

### Nested JSON fields

```go
_ = s.Set("user:1", `{"name":"Alice","addr":{"city":"Taipei"}}`, store.SetDefault, nil)

val, _ := s.GetField("user:1", []string{"addr", "city"})
_ = s.SetField("user:1", []string{"addr", "zip"}, "100", store.SetDefault, nil)
_ = s.DelField("user:1", []string{"addr", "zip"})

_, _ = s.IncrField("user:1", []string{"age"}, 1)
```

### Find global value lookup

```go
import "github.com/pardnchiu/ToriiDB/core/store/filter"

keys := s.Find(filter.EqualTo, "Alice", 0)
keys = s.Find(filter.GreaterThan, "50", 10)
keys = s.Find(filter.Like, "*error*", 0)
```

### Query via struct composition

```go
import "github.com/pardnchiu/ToriiDB/core/store/filter"

results := s.Query(filter.And{
    filter.GTE{Field: "age", Value: "18"},
    filter.LT{Field: "age", Value: "65"},
    filter.Or{
        filter.EQ{Field: "dept", Value: "engineering"},
        filter.EQ{Field: "dept", Value: "design"},
    },
    filter.Not{filter.EQ{Field: "status", Value: "banned"}},
}, 10)

nested := s.Query(filter.EQ{Field: "addr.city", Value: "Taipei"}, 0)
```

### Query via string expression

```go
f, err := filter.AtoFilter("(age GT 20 AND age LT 30) OR score >= 90")
if err != nil {
    log.Fatal(err)
}
results := s.Query(f, 0)
```

String expressions are built for concatenating untrusted input:

```go
f, _ = filter.AtoFilter(
    fmt.Sprintf("tool EQ %s AND symptom LIKE *%s*", tool, keyword),
)
```

### Semantic vector search

```go
ctx := context.Background()

_ = s.SetVector(ctx, "doc:1", "How to cook pasta", store.SetDefault, nil)
_ = s.SetVector(ctx, "doc:2", "Building a web server in Go", store.SetDefault, nil)
_ = s.SetVector(ctx, "doc:3", "Italian dinner recipes", store.SetDefault, nil)

keys, err := s.VSearch(ctx, "cooking recipe", "doc:*", 2)
if err != nil {
    log.Fatal(err)
}
// keys == []string{"doc:3", "doc:1"} — top-K by cosine, descending

score, err := s.VSim("doc:1", "doc:3")
// score ~= 0.82 (cosine similarity, -1..1)

vec, ok := s.VGet("doc:1") // defensive copy of []float32
```

### REPL session

```bash
go run ./cmd/test
```

```text
toriidb[0]> SET user:1 {"name":"Alice","age":25}
OK
toriidb[0]> GET user:1.name
Alice
toriidb[0]> QUERY age GT 20 AND name LIKE Ali*
1) user:1
toriidb[0]> SET doc:1 "How to cook pasta" VECTOR
OK
toriidb[0]> SET doc:2 "Italian dinner recipes" VECTOR
OK
toriidb[0]> VSEARCH cooking recipe MATCH doc:* LIMIT 2
1) doc:2
2) doc:1
toriidb[0]> VSIM doc:1 doc:2
(float) 0.8213
toriidb[0]> SELECT 3
OK
toriidb[3]> exit
```

## API Reference

### Core types

| Type | Description |
|------|-------------|
| `Store` | Top-level handle owning all 16 DBs and the background expiration goroutine |
| `Session` | Derived from `Store.Session()`; shares data but owns an independent db index |
| `Entry` | A single record; `value` and `parsed` are private and only mutated via methods |
| `ValueType` | Enum auto-detected on write (JSON / String / Int / Float / Bool / Date) |
| `SetFlag` | `SetDefault` / `SetNX` / `SetXX` |

### Store lifecycle

| Function | Signature | Description |
|----------|-----------|-------------|
| `New` | `func New(path ...string) (*Store, error)` | Creates a Store; no arg uses `./temp`, one arg uses the given directory |
| `(*Store).Close` | `func (s *Store) Close() error` | Cancels the cleanTimer and compacts every loaded DB's AOF in parallel |
| `(*Store).Session` | `func (s *Store) Session() *Session` | Spawns a Session with its own db index |

### DB switching

| Method | Signature | Description |
|--------|-----------|-------------|
| `Select` | `func (c *core) Select(index int) error` | Switches DB; valid range 0-15 |
| `Current` | `func (c *core) Current() int` | Returns the current db index |
| `DB` | `func (c *core) DB() *db` | Returns the current db, triggering lazy AOF replay on first call |

### Reads and writes

| Method | Signature | Description |
|--------|-----------|-------------|
| `Set` | `func (c *core) Set(key, value string, flag SetFlag, expireAt *int64) error` | Writes; JSON values warm the parsed cache |
| `Get` | `func (c *core) Get(key string) (*Entry, bool)` | Reads; lazy-deletes expired entries |
| `SetField` | `func (c *core) SetField(key string, subKeys []string, value string, flag SetFlag, expireAt *int64) error` | Writes a nested field via dot-notation |
| `GetField` | Returns the raw string of a nested field | |
| `Del` | `func (c *core) Del(keys ...string) int` | Batch delete; returns the actual count |
| `DelField` | `func (c *core) DelField(key string, subKeys []string) error` | Deletes a nested field |
| `Exist` / `ExistField` | Return `(integer) 0/1` strings | |
| `Type` / `TypeField` | Return a type label (e.g., `json`) | |
| `Incr` | `func (c *core) Incr(key string, delta float64) (float64, error)` | Numeric increment |
| `IncrField` | `func (c *core) IncrField(key string, subKeys []string, delta float64) (float64, error)` | Nested numeric increment |
| `Keys` | `func (c *core) Keys(pattern string) []string` | Glob matching |

### Expiration

| Method | Signature | Description |
|--------|-----------|-------------|
| `TTL` | `func (c *core) TTL(key string) int64` | Remaining seconds; `-1` no expiration, `-2` missing |
| `Expire` | `func (c *core) Expire(key string, seconds int64) error` | Expire after N seconds |
| `ExpireAt` | `func (c *core) ExpireAt(key string, ts int64) error` | Expire at a Unix timestamp |
| `Persist` | `func (c *core) Persist(key string) error` | Removes expiration |

### Queries

| Method | Signature | Description |
|--------|-----------|-------------|
| `Find` | `func (c *core) Find(op filter.Operator, value string, limit int) []string` | Global value comparison; auto-shards past 1024 entries |
| `Query` | `func (c *core) Query(f filter.Filter, limit int) []string` | JSON field predicate; accepts any `filter.Filter` |
| `Exec` | `func (c *core) Exec(input string) string` | REPL command router |

### Vector

| Method | Signature | Description |
|--------|-----------|-------------|
| `SetVector` | `func (c *core) SetVector(ctx context.Context, key, value string, flag SetFlag, expireAt *int64) error` | Writes the main key synchronously, then asynchronously attaches the embedding via a background goroutine (joined on `Close()`) |
| `VSearch` | `func (c *core) VSearch(ctx context.Context, text, pattern string, k int) ([]string, error)` | Top-K cosine search; `k <= 0` defaults to 10; `pattern` filters keys via glob match; skips internal `__torii:*` keys |
| `VSim` | `func (c *core) VSim(key1, key2 string) (float64, error)` | Cosine similarity between two stored vectors; returns `errVectorMissing` on any missing / empty vector, `errVectorMismatch` on dimension mismatch |
| `VGet` | `func (c *core) VGet(key string) ([]float32, bool)` | Returns a defensive copy of the stored vector so callers cannot mutate `Entry.Vector` |

### filter package

| Type | Description |
|------|-------------|
| `Filter` | Core interface `Match(obj any) bool` |
| `Cond` | `{Field, Op, Value}` base predicate |
| `And` / `Or` / `Not` | Logical combinators |
| `EQ` / `NE` / `GT` / `GTE` / `GE` / `LT` / `LTE` / `LE` / `LIKE` | Sugar predicates |
| `Operator` | Enum (`EqualTo`, `NotEqualTo`, `GreaterThan`, `GreaterThanOrEqualTo`, `LessThan`, `LessThanOrEqualTo`, `Like`) |
| `AtoFilter` | `func AtoFilter(str string) (Filter, error)` — parses a string expression |
| `AtoOperation` | `func AtoOperation(s string) (Operator, bool)` — string to operator |
| `Match` | `func Match(stored string, op Operator, target string) bool` — raw value compare |

### REPL commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `GET` | `GET <key[.field...]>` | Reads a key or nested field |
| `SET` | `SET <key> <value> [NX\|XX] [<seconds>] [VECTOR]` | Writes; trailing integer is TTL seconds; trailing `VECTOR` attaches an OpenAI embedding |
| `DEL` | `DEL <key> [key2...]` or `DEL <key.field>` | Batch delete or nested field delete |
| `EXIST` | `EXIST <key[.field...]>` | Returns `(integer) 0/1` |
| `TYPE` | `TYPE <key[.field...]>` | Returns the type label |
| `INCR` | `INCR <key[.field...]> [delta]` | Delta defaults to 1 |
| `TTL` / `EXPIRE` / `EXPIREAT` / `PERSIST` | Expiration control | |
| `KEYS` | `KEYS <pattern>` | Glob matching |
| `FIND` | `FIND <op> <value> [LIMIT <n>]` | Global value search |
| `QUERY` | `QUERY <expression> [LIMIT <n>]` | Infix expression query |
| `VSEARCH` | `VSEARCH <text> [MATCH <pattern>] [LIMIT <n>]` | Top-K cosine search; `MATCH` / `LIMIT` can appear in either order; default `LIMIT 10` |
| `VSIM` | `VSIM <key1> <key2>` | Cosine similarity between two stored vectors; `(nil)` on missing / empty vector |
| `VGET` | `VGET <key>` | Returns the stored vector as a JSON array (debug helper) |
| `SELECT` | `SELECT <0-15>` | Switches DB |
| `exit` / `quit` | Leaves the REPL | |

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
