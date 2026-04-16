# ToriiDB - Architecture

> Back to [README](../README.md)

## Overview

```mermaid
graph TB
    Client[REPL / Embed API] --> Exec[Exec Router]
    Exec --> Core[core - db index]
    Core --> Store[Store - 16 DBs]
    Core --> Filter[Filter Engine]

    Store --> Memory[In-Memory Map]
    Store --> AOF[AOF Append Log]
    Store --> Snapshot[MD5 JSON Snapshot]

    Filter --> Scan[Chunked Parallel Scan]
    Scan --> Memory

    BG[cleanTimer goroutine] -. ctx cancel .-> Store
```

Core object relationships:

- `Store` owns the `[16]*db` array and the `cleanTimer` context cancel.
- `core` is the struct embedded by both `Store` and `Session`, holding a pointer to `Store.allDBs` and the current db index.
- `Session` is spawned by `Store.Session()`, sharing the underlying db array while owning its own index.
- The `filter` package is independent from `store`, consumed only through the `Filter` interface in `Query`.

## Module: Store

Owns the database lifecycle, directory layout, and background expiration sweeper.

```mermaid
graph TB
    subgraph Store
        New[New path...] --> Alloc[allocate 16 dbs]
        Alloc --> Timer[start cleanTimer]
        Close[Close] --> Cancel[cancel context]
        Cancel --> Parallel[compact every loaded db in parallel]
        Session[Session] --> Derive[spawn core sharing allDBs]
    end
    FS[(local filesystem)] --> Alloc
    Timer --> Clean[cleanExpired per-db]
```

- `New(path ...string)`: validates the directory, allocates `[16]*db`, and starts the background goroutine that runs `cleanExpired` every minute.
- `Close()`: cancels the context so `cleanTimer` exits, then uses a `sync.WaitGroup` to compact every `loaded` db in parallel.
- `Session()`: clones `core` so upper-layer goroutines can switch databases without affecting the original Store.

## Module: db

Per-database memory state and persistence carriers.

```mermaid
graph TB
    subgraph db
        Data[data: map string *Entry]
        Mu[sync.RWMutex]
        AOFFile[aof *os.File]
        Once[sync.Once]
        Loaded[loaded bool]
        Size[aofSize / aofSizeBaseline]
    end

    Write[Set / SetField / Del / Expire...] --> Mu
    Mu --> Data
    Write --> AOFAppend[addToAOF]
    AOFAppend --> AOFFile
    AOFAppend --> Inflate{size >= baseline * 2?}
    Inflate -- yes --> Compact[compact]
    Inflate -- no --> OK[write done]

    First[first access] --> Once
    Once --> Replay[replayAOF]
    Replay --> Data
```

- `ensureLoaded`: `sync.Once` guarantees AOF replay only runs once, so pre-access startup cost is zero.
- `init`: lazily creates the AOF file, opening `record.aof` only on the first write.
- `compact`: closes the current AOF, remarshals non-expired entries, and atomically replaces the file via `utils.WriteFile`.
- `cleanExpired`: scans `data`, drops entries whose `ExpireAt <= now`, and removes their JSON snapshot files.

## Module: Entry

Represents both in-memory state and the on-disk JSON snapshot format, while maintaining a parsed cache.

```mermaid
classDiagram
    class Entry {
        +string Key
        -string value
        +ValueType Type
        +int64 CreatedAt
        +*int64 UpdatedAt
        +*int64 ExpireAt
        -any parsed
        +Value() string
        -setValue(v)
        -setParsed(obj)
        +parseAndCache() (any, bool)
        +cached() (any, bool)
        +JSON() ([]byte, error)
    }
    Entry ..> ValueType : type
```

Lock discipline:

- `parseAndCache()` mutates `e.parsed`, so callers must hold the write lock or run single-threaded (`Set` / `SetField` / `IncrField` / `DelField` / AOF replay).
- `cached()` only reads `e.parsed` and is safe to call under an RLock (`Query` / `GetField`).
- Every write path must warm `parsed` before releasing the write lock so readers always see a populated cache.

## Module: Exec

The single routing point for REPL commands, parsing string input into `core` method calls.

```mermaid
graph LR
    Input[input string] --> Fields[strings.Fields]
    Fields --> Switch{first token}
    Switch -->|GET/SET/DEL/INCR| KV[splitKey + core method]
    Switch -->|EXPIRE/TTL/PERSIST| TTL[expiration control]
    Switch -->|KEYS| Glob[Keys pattern]
    Switch -->|FIND| Find[parseLimit + filter.AtoOperation]
    Switch -->|QUERY| Query[extractLimitFromInfix + filter.AtoFilter]
    Switch -->|SELECT| Select[Select index]
    KV --> Core
    TTL --> Core
    Glob --> Core
    Find --> Core
    Query --> Core
    Select --> Core
```

- `splitKey` cuts on the first `.` into main key + sub-keys; without a `.`, the plain KV path runs.
- `parseSetArgs` walks the args backwards: a trailing integer is treated as TTL seconds, and a preceding `NX`/`XX` becomes the flag.
- `extractLimitFromInfix` and `parseLimit` strip `LIMIT <n>` from the tail of the expression.

## Module: filter

The shared predicate engine under `Query`, also shipping a string-expression parser.

```mermaid
graph TB
    subgraph filter
        Ifc[Filter interface: Match obj any bool]
        Cond[Cond Field Op Value]
        Sugar[EQ / NE / GT / GTE / GE / LT / LTE / LE / LIKE]
        Logic[And / Or / Not]
        Op[Operator enum]
        Match[Match stored op target]
        AtoOp[AtoOperation str]
        Parser[Parser Words Position]
        AtoFilter[AtoFilter str]
    end

    Sugar --> Ifc
    Cond --> Ifc
    Logic --> Ifc
    Cond --> Match
    AtoOp --> Op
    AtoFilter --> Parser
    Parser --> Logic
    Parser --> Cond
```

- `Parser` is recursive-descent: `Or` → `And` → `Not` → `Primary`, with parentheses and base predicates handled inside `Primary`.
- `AtoFilter` first peels leading `(` and trailing `)` into standalone tokens, then hands the token list to `Parser.Or()` to build the AST.
- `Match` accepts both numeric and string values — numeric comparison first tries `utils.Vtof`, falling back to string comparison on failure.

## Data Flow: Set → Persistence

```mermaid
sequenceDiagram
    participant Caller
    participant core
    participant db
    participant Entry
    participant FS as Filesystem

    Caller->>core: Set key value flag expireAt
    core->>db: DB().mu.Lock
    core->>Entry: setValue or new Entry
    alt type is JSON
        core->>Entry: parseAndCache
    end
    core->>Entry: JSON
    core->>FS: utils.WriteFile snapshot
    core->>db: addToAOF SET
    db->>FS: append + fsync
    alt aofSize >= baseline * 2
        db->>db: compact inline
    end
    core-->>Caller: nil / error
```

## Data Flow: Query → Chunked Parallelism

```mermaid
sequenceDiagram
    participant Caller
    participant core
    participant db
    participant Scan as sliceScan
    participant Filter

    Caller->>core: Query filter limit
    core->>db: DB().mu.RLock
    core->>db: gather all keys
    alt len keys > 1024
        core->>Scan: split into chunks
        par one goroutine per chunk
            Scan->>Filter: Match cached entry
        end
        Scan-->>core: merge shards
    else
        core->>Filter: Match each entry
    end
    core->>core: sortAndCollect limit
    core-->>Caller: []string keys
```

## State Machine: db lifecycle

```mermaid
stateDiagram-v2
    [*] --> Initialized: New allocated
    Initialized --> Loaded: first access ensureLoaded
    Loaded --> Active: accept reads/writes
    Active --> Active: addToAOF + snapshot
    Active --> Compacted: aofSize >= baseline * 2 or Close
    Compacted --> Active: accept writes again
    Active --> [*]: Store.Close + compact
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
