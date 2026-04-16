# ToriiDB - 架構

> 返回 [README](./README.zh.md)

## 概覽

```mermaid
graph TB
    Client[REPL / Embed API] --> Exec[Exec 路由]
    Exec --> Core[core - db index]
    Core --> Store[Store - 16 DBs]
    Core --> Filter[Filter 引擎]

    Store --> Memory[記憶體 Map]
    Store --> AOF[AOF 追加日誌]
    Store --> Snapshot[MD5 JSON 快照]

    Filter --> Scan[切塊並發掃描]
    Scan --> Memory

    BG[cleanTimer goroutine] -. ctx 取消 .-> Store
```

核心物件關係：

- `Store` 擁有 `[16]*db` 陣列與 `cleanTimer` 的 context cancel。
- `core` 是 `Store` 與 `Session` 的嵌入結構，持有指向 `Store.allDBs` 的指標與目前 db index。
- `Session` 由 `Store.Session()` 衍生，共享底層 db 陣列但擁有獨立 index。
- `filter` 套件獨立於 store，僅透過 `Filter` 介面被 `Query` 呼叫。

## Module: Store

負責資料庫生命週期、目錄配置與背景過期清理。

```mermaid
graph TB
    subgraph Store
        New[New path...] --> Alloc[配置 16 個 db]
        Alloc --> Timer[啟動 cleanTimer]
        Close[Close] --> Cancel[cancel context]
        Cancel --> Parallel[並行 compact 所有 loaded db]
        Session[Session] --> Derive[衍生 core, 共享 allDBs]
    end
    FS[(本地檔案系統)] --> Alloc
    Timer --> Clean[cleanExpired per-db]
```

- `New(path ...string)`：驗證目錄後配置 `[16]*db`，啟動每分鐘執行 `cleanExpired` 的背景 goroutine。
- `Close()`：取消 context 讓 `cleanTimer` 結束，並以 `sync.WaitGroup` 並行壓縮每個 `loaded` 的 db。
- `Session()`：複製 `core`，讓上層 goroutine 切換 db 不影響原 Store。

## Module: db

單一資料庫的記憶體狀態與持久化載體。

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
    Inflate -- no --> OK[寫入完成]

    First[第一次讀寫] --> Once
    Once --> Replay[replayAOF]
    Replay --> Data
```

- `ensureLoaded`：`sync.Once` 保證 AOF 只 replay 一次，首次存取前啟動成本為零。
- `init`：延遲建立 AOF 檔，僅在第一次寫入時打開 `record.aof`。
- `compact`：關閉目前 AOF、將非過期 entry 重新 marshal 後透過 `utils.WriteFile` atomically 替換。
- `cleanExpired`：掃描 `data`，刪除 `ExpireAt <= now` 的記錄並一併移除對應 JSON 快照檔。

## Module: Entry

同時代表記憶體狀態與 JSON 快照格式，並維護 parsed 快取。

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

鎖紀律：

- `parseAndCache()` 會寫入 `e.parsed`，呼叫者必須持有寫鎖或處於單執行緒路徑（`Set` / `SetField` / `IncrField` / `DelField` / AOF replay）。
- `cached()` 僅讀取 `e.parsed`，安全於 RLock 下呼叫（`Query` / `GetField`）。
- 每個寫入路徑在釋放寫鎖之前必須先 warm `parsed`，確保讀取端永遠能命中快取。

## Module: Exec

REPL 命令的單一路由點，將字串輸入解析成 `core` 方法呼叫。

```mermaid
graph LR
    Input[輸入字串] --> Fields[strings.Fields]
    Fields --> Switch{第一個 token}
    Switch -->|GET/SET/DEL/INCR| KV[splitKey + core method]
    Switch -->|EXPIRE/TTL/PERSIST| TTL[過期控制]
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

- `splitKey` 以首個 `.` 切分成主 key 與子 key 列表，無 `.` 時走一般 KV 路徑。
- `parseSetArgs` 從尾端倒著解析：最後一個整數視為 TTL 秒數、倒數第二個 `NX`/`XX` 視為 flag。
- `extractLimitFromInfix` 與 `parseLimit` 負責將 `LIMIT <n>` 從表達式尾端剝離。

## Module: filter

`Query` 底層共用的條件匹配引擎，同時提供字串表達式解析。

```mermaid
graph TB
    subgraph filter
        Ifc[Filter interface: Match obj any bool]
        Cond[Cond Field Op Value]
        Sugar[EQ / NE / GT / GTE / GE / LT / LTE / LE / LIKE]
        Logic[And / Or / Not]
        Op[Operator 列舉]
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

- `Parser` 以遞迴下降實作：`Or` → `And` → `Not` → `Primary`，於 `Primary` 中處理括號與基本條件。
- `AtoFilter` 先將 `(` / `)` 從 token 中剝離成獨立詞，再交給 `Parser.Or()` 建構 AST。
- `Match` 同時接受數值與字串，數值比較先走 `utils.Vtof`，失敗後退回字串比較。

## Data Flow: Set → 持久化

```mermaid
sequenceDiagram
    participant Caller
    participant core
    participant db
    participant Entry
    participant FS as 檔案系統

    Caller->>core: Set key value flag expireAt
    core->>db: DB().mu.Lock
    core->>Entry: setValue 或 new Entry
    alt 型別為 JSON
        core->>Entry: parseAndCache
    end
    core->>Entry: JSON
    core->>FS: utils.WriteFile 快照
    core->>db: addToAOF SET
    db->>FS: append + fsync
    alt aofSize >= baseline * 2
        db->>db: compact inline
    end
    core-->>Caller: nil / error
```

## Data Flow: Query → 切塊並發

```mermaid
sequenceDiagram
    participant Caller
    participant core
    participant db
    participant Scan as sliceScan
    participant Filter

    Caller->>core: Query filter limit
    core->>db: DB().mu.RLock
    core->>db: 收集所有 key
    alt len keys > 1024
        core->>Scan: 切塊
        par 每塊一個 goroutine
            Scan->>Filter: Match cached entry
        end
        Scan-->>core: 合併結果
    else
        core->>Filter: Match 每個 entry
    end
    core->>core: sortAndCollect limit
    core-->>Caller: []string keys
```

## State Machine: db 生命週期

```mermaid
stateDiagram-v2
    [*] --> Initialized: New 配置完成
    Initialized --> Loaded: 首次讀寫 ensureLoaded
    Loaded --> Active: 接受讀寫
    Active --> Active: addToAOF + 快照
    Active --> Compacted: aofSize >= baseline * 2 或 Close
    Compacted --> Active: 繼續接受寫入
    Active --> [*]: Store.Close + compact
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
