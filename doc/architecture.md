# ToriiDB - Architecture

> Back to [README](../README.md)

## Overview

```mermaid
graph TB
    Client[REPL / Embed API] --> Exec[Exec Router]
    Exec --> Core[core - db index + embedder]
    Core --> Store[Store - 16 DBs]
    Core --> Filter[Filter Engine]
    Core --> Vector[Vector Engine]

    Store --> Memory[In-Memory Map]
    Store --> AOF[AOF Append Log]
    Store --> Snapshot[MD5 JSON Snapshot]

    Filter --> Scan[Chunked Parallel Scan]
    Scan --> Memory

    Vector --> TopK[Top-K Cosine Scan]
    Vector --> EmbedCache[__torii:embed cache]
    Vector --> OpenAI[OpenAI Client]
    TopK --> Memory
    EmbedCache --> Memory

    BG[cleanTimer goroutine] -. ctx cancel .-> Store
    WG[Store.wg] -. join background embeds .-> Core
```

Core object relationships:

- `Store` owns the `[16]*db` array, the `cleanTimer` context cancel, and the `sync.WaitGroup` that tracks in-flight async vector attaches.
- `core` is the struct embedded by both `Store` and `Session`, holding a pointer to `Store.allDBs`, the current db index, and the shared `*openai.Client` (nil if `OPENAI_API_KEY` is not set).
- `Session` is spawned by `Store.Session()`, sharing the underlying db array, embedder, and WaitGroup while owning its own index.
- The `filter` package is independent from `store`, consumed only through the `Filter` interface in `Query`.
- The vector path is opt-in: `SetVector` / `VSearch` no-op with an explicit error when the embedder is nil, so the core KV path stays dependency-free at runtime.

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

## Module: Vector

Vector persistence lives inline on each `Entry`; the only sidecar is the embedding cache under the `__torii:embed:*` prefix.

```mermaid
graph TB
    subgraph Vector
        SetV[SetVector] --> MainWrite[main key write + AOF]
        SetV --> Async[background goroutine]
        Async --> WG[Store.wg]
        Async --> CacheLookup{embed cache HIT?}
        CacheLookup -- yes --> AttachVec[attach Entry.Vector]
        CacheLookup -- no --> Embed[OpenAI embed]
        Embed --> CachePut[__torii:embed:sha256 put]
        CachePut --> AttachVec
        AttachVec --> AOFV[addToAOFWithVector]

        VSearch[VSearch] --> QEmbed[resolveQueryVector cache HIT / embed]
        QEmbed --> ScanTopK[scan d.data + min-heap]
        ScanTopK --> Output[top-K keys]

        VSim[VSim] --> PairRead[Get key1 + Get key2]
        PairRead --> Cos[cosine]

        VGet[VGet] --> Copy[defensive copy of Entry.Vector]
    end
    OpenAI[OpenAI HTTP] <--> Embed
    FS[(AOF)] <-- replay/compact --> AOFV
```

- `Entry.Vector []float32`: inline per-key embedding, `nil` when absent.
- `vector.go`: base64 little-endian float32 codec (`encodeVector` / `decodeVector`), `cosine`, and `isInternal` — any key with the reserved `__torii:` prefix is skipped by scan commands.
- `vcache.go`: `getVector` / `putVector` store cached embeddings under `__torii:embed:<sha256(model|dim|text)>`. Payload is JSON `{"v":"<base64>","d":<dim>,"m":"<model>"}`. `d != currentDim` is treated as MISS; no TTL since embeddings are deterministic per (model, dim, text).
- `aof.go`: `AOFRecord.Vector *string` persists the base64 vector; emitted only when `len(vec) > 0` for backward compatibility. `replayAOF` decodes back into `Entry.Vector`; `compact` re-emits.
- `SetVector` lock order: main key write under write lock → AOF → release; background goroutine reads `__torii:embed:*` under RLock, calls OpenAI under **no lock**, then takes the write lock twice (once to put cache, once to attach to main key + append AOF).
- Plain `Set()` invalidates `Entry.Vector` on overwrite — a re-set without `VECTOR` means the underlying text changed, so the stale embedding is dropped.
- `VSearch` holds the db RLock across the whole scan; `scanTopK` maintains a size-k min-heap so worst case is O(n log k).
- `Close()` blocks on `Store.wg` before compacting AOF, guaranteeing no in-flight embed races the shutdown.

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

## Data Flow: SetVector → async attach

```mermaid
sequenceDiagram
    participant Caller
    participant core
    participant db
    participant BG as goroutine
    participant OpenAI
    participant FS as Filesystem

    Caller->>core: SetVector ctx key value flag expireAt
    core->>db: mu.Lock main key write
    core->>FS: snapshot + addToAOF
    core-->>Caller: nil (main key durable)
    core->>BG: go attachVectorBG Store.wg.Add
    BG->>db: mu.RLock + getVector from __torii:embed
    alt cache HIT
        db-->>BG: []float32
    else cache MISS
        BG->>OpenAI: POST /v1/embeddings
        OpenAI-->>BG: []float32
        BG->>db: mu.Lock putVector + addToAOFWithVector
    end
    BG->>db: mu.Lock attach Entry.Vector + addToAOFWithVector
    BG-->>core: Store.wg.Done
```

## Data Flow: VSearch → top-K cosine

```mermaid
sequenceDiagram
    participant Caller
    participant core
    participant db
    participant Cache as __torii:embed
    participant OpenAI

    Caller->>core: VSearch ctx text pattern k
    core->>db: mu.RLock resolveQueryVector
    core->>Cache: getVector model dim text
    alt cache HIT
        Cache-->>core: []float32
    else cache MISS
        core->>OpenAI: POST /v1/embeddings
        OpenAI-->>core: []float32
        core->>db: mu.Lock putVector
    end
    core->>db: mu.RLock scanTopK
    loop each entry
        core->>core: skip isInternal / expired / dim mismatch / pattern no-match
        core->>core: cosine + min-heap Push/Fix
    end
    core-->>Caller: top-K keys descending
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
