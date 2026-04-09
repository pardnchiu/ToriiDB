# ToriiDB

> 專案 [go-redis-fallback](https://github.com/pardnchiu/go-redis-fallback) 的延伸<br>
> 聚焦在以 JSON 為核心的資料庫，實現 CRUD、Redis 快取與過期淘汰，以及 MongoDB 的基本查找<br>
> 只以標準庫實現，目標是取代 SQLite 作為 Golang 中小型專案的資料庫基礎

## 適用場景

零依賴、純 Go 實作，不需要安裝任何外部資料庫。
適用於不需要真實資料庫、只需要檔案持久化加記憶體快取的專案。
資料持久化至磁碟（AOF + JSON 快取），記憶體中保留快取加速讀取。
無索引設計，所有搜尋皆為全掃描；`GET`/`SET` 為 O(1)，`FIND`/`QUERY` 為 O(n)。
超過 1024 筆時自動切塊並發掃描，萬筆級複合查詢實測 ~3ms（Apple M5），遠低於 50ms。
適合資料量在萬筆以內、單機單進程的場景：

- CLI 工具 — 本地設定與狀態儲存
- Prototype / MVP — 省去架設 DB 的時間，直接嵌入
- 桌面應用、IoT 裝置 — 嵌入式儲存，交叉編譯友善
- Line Bot、Discord Bot — 對話狀態與用戶資料
- AI 個人助理 — 記憶儲存、對話歷史、偏好設定、上下文快取
- 單機 API Server — Session、Token、設定檔等輕量儲存
- 個人部落格、CMS 後台 — 千篇文章、萬名用戶以內
- 量級用不到 Redis / MongoDB / SQLite 的中小型專案

**不適合**：高併發線上服務、資料量超過記憶體、跨機器存取、多進程同時寫入、需要索引加速的大量查詢場景。

專案本質是 KV 儲存，搜尋透過全掃描與值的型別判斷實現，非傳統資料庫引擎。後續將提供匯出至 MongoDB 的工具函式，資料結構天然相容。

## 已完成功能

### 2026-04-10

- 抽離 `core` 結構體，命令方法由 `Store` 與 `Session` 共用
- 新增 `Session` 類型，擁有獨立 db index
- `New(path ...string)` 支援自訂儲存路徑，含目錄驗證與自動建立
- DB lazy replay：首次存取才載入 AOF，啟動不再掃描全部 16 個 DB
- `Close()` 並行 compact，shutdown 耗時降為最慢單一 DB
- 以 `context.Context` 取代無緩衝 channel 取消 cleanTimer，消除潛在 deadlock
- 修正 `Expire`/`ExpireAt`/`Persist` 未同步寫入 JSON 快取檔案
- 修正 `cleanExpired` 未刪除對應 JSON 快取檔案

### 2026-04-09

- `FIND`/`QUERY` 新增 NE（不等於）比較運算子
- `QUERY` 支援中綴表達式：AND / OR / NOT 邏輯運算子與括號分組
- `QUERY` 支援擴展運算子別名：GTE / LTE / >= / <= / !=
- 提取獨立 `filter` 套件，統一 FIND/QUERY 的運算子與匹配邏輯
- `FIND`/`QUERY` 超過 1024 筆自動切塊並發掃描

### 2026-04-08

- `KEYS` glob 模式匹配
- `GET`/`SET`/`DEL` 支援 dot-notation 巢狀欄位存取
- `EXIST`/`TYPE` 支援 dot-notation 巢狀欄位查詢
- `INCR` 數值遞增（獨立 key 與巢狀 JSON 欄位）
- `FIND` 全域值搜尋（EQ / GT / GE / LT / LE / LIKE）
- `QUERY` JSON 子欄位條件查詢（dot-notation + EQ / GT / GE / LT / LE / LIKE）
- AOF compaction on close（atomic file write）
- 修正無寫入 session 時 AOF compaction 被跳過的問題

### 2026-04-07

- 記憶體儲存（per-DB 獨立）
- MD5 三層目錄 JSON 快取
- AOF 持久化與啟動 replay
- `SET` 支援 NX/XX flag 與可選 TTL
- `GET` 記憶體讀取與過期檢查
- `DEL` 批次刪除
- `EXIST` 存在檢查
- `TYPE` 值類型查詢
- `TTL` 查詢剩餘秒數
- `EXPIRE` 設定過期秒數
- `EXPIREAT` 設定過期時間戳
- `PERSIST` 移除過期設定
- Lazy delete（GET 時過期清除）
- 背景 Goroutine 定期清理過期 key
- `SELECT` 切換 DB 0-15
- 每個 DB 獨立記憶體空間與 AOF
- 延遲建立目錄與檔案
- 值類型自動偵測（JSON / String / Int / Float / Bool / Date）

## Usage

### 初始化

```go
import "github.com/pardnchiu/ToriiDB/core/store"

// 預設儲存路徑 (./temp)
s, err := store.New()

// 自訂儲存路徑
s, err := store.New("/data/torii")

defer s.Close()
```

### 切換資料庫

```go
// 共 16 個資料庫 (0-15)，首次存取時才載入
s.Select(6)
s.Current() // 6
```

### Session

```go
// Session 共享相同資料，但擁有獨立的 db index
sess := s.Session()
sess.Select(3)
sess.Current() // 3
s.Current()    // 6（不受影響）
```

### Set / Get / Del

```go
// 基本寫入
s.Set("user:1", `{"name":"Alice","age":25}`, store.SetDefault, nil)

// NX：僅在 key 不存在時寫入
s.Set("user:1", "value", store.SetNX, nil)

// 帶 TTL：60 秒後過期
exp := time.Now().Unix() + 60
s.Set("token:abc", "session_data", store.SetDefault, &exp)

// 讀取
entry, ok := s.Get("user:1")
if ok {
    fmt.Println(entry.Value) // {"name":"Alice","age":25}
    fmt.Println(entry.Type)  // json
}

// 刪除（回傳實際刪除的數量）
count := s.Del("user:1", "user:2")
```

### 巢狀欄位存取（dot-notation）

```go
s.Set("user:1", `{"name":"Alice","addr":{"city":"Taipei"}}`, store.SetDefault, nil)

// 讀取巢狀欄位
val, ok := s.GetField("user:1", []string{"addr", "city"})
// val = "Taipei"

// 寫入巢狀欄位
s.SetField("user:1", []string{"addr", "zip"}, "100", store.SetDefault, nil)

// 刪除巢狀欄位
s.DelField("user:1", []string{"addr", "zip"})

// 檢查存在 / 型別
s.Exist("user:1")                            // "(integer) 1"
s.ExistField("user:1", []string{"addr"})     // "(integer) 1"
s.Type("user:1")                             // "json"
s.TypeField("user:1", []string{"addr"})      // "json"
```

### 數值遞增

```go
s.Set("counter", "10", store.SetDefault, nil)
result, err := s.Incr("counter", 1)   // 11
result, err = s.Incr("counter", -3)   // 8
result, err = s.Incr("counter", 0.5)  // 8.5

// 巢狀欄位遞增
s.Set("stats", `{"views":100}`, store.SetDefault, nil)
result, err = s.IncrField("stats", []string{"views"}, 1) // 101
```

### 過期控制

```go
s.TTL("key")          // 剩餘秒數，-1 = 無過期，-2 = key 不存在
s.Expire("key", 300)  // 300 秒後過期
s.ExpireAt("key", ts) // 指定 Unix 時間戳過期
s.Persist("key")      // 移除過期設定
```

### Keys 模式匹配

```go
s.Keys("*")       // 所有 key
s.Keys("user:*")  // 符合 glob 模式的 key
```

### Find 全域值搜尋

```go
import "github.com/pardnchiu/ToriiDB/core/store/filter"

// 跨所有 key 搜尋值（回傳符合的 key 列表）
s.Find(filter.EqualTo, "Alice", 0)              // 精確匹配，無上限
s.Find(filter.GreaterThan, "50", 10)             // 值 > 50，最多 10 筆
s.Find(filter.Like, "*error*", 0)                // 包含 "error"
```

| 運算子 | 別名 |
|---|---|
| `filter.EqualTo` | EQ, = |
| `filter.NotEqualTo` | NE, != |
| `filter.GreaterThan` | GT, > |
| `filter.GreaterThanOrEqualTo` | GTE, GE, >= |
| `filter.LessThan` | LT, < |
| `filter.LessThanOrEqualTo` | LTE, LE, <= |
| `filter.Like` | LIKE |

### Query JSON 欄位查詢

針對 JSON 值的欄位條件查詢，提供兩種風格：

#### 結構體組合（型別安全，適合程式內建構）

```go
import "github.com/pardnchiu/ToriiDB/core/store/filter"

// 單一條件
results := s.Query(filter.GE{Field: "age", Value: "18"}, 0)

// AND 組合
results = s.Query(filter.And{
    filter.GTE{Field: "age", Value: "18"},
    filter.LT{Field: "age", Value: "65"},
}, 0)

// OR 組合
results = s.Query(filter.Or{
    filter.EQ{Field: "status", Value: "active"},
    filter.EQ{Field: "role", Value: "admin"},
}, 0)

// NOT 反轉
results = s.Query(filter.Not{
    filter.EQ{Field: "status", Value: "banned"},
}, 0)

// 複合條件
results = s.Query(filter.And{
    filter.GTE{Field: "score", Value: "60"},
    filter.Or{
        filter.EQ{Field: "dept", Value: "engineering"},
        filter.EQ{Field: "dept", Value: "design"},
    },
    filter.Not{filter.EQ{Field: "status", Value: "inactive"}},
}, 10)

// 巢狀欄位（dot-notation）
results = s.Query(filter.EQ{Field: "addr.city", Value: "Taipei"}, 0)
```

#### 字串表達式（動態建構，適合外部輸入）

```go
import "github.com/pardnchiu/ToriiDB/core/store/filter"

// 單一條件
f, _ := filter.AtoFilter("age GTE 18")
results := s.Query(f, 0)

// AND / OR / NOT
f, _ = filter.AtoFilter("age >= 18 AND age < 65")
f, _ = filter.AtoFilter("status EQ active OR role = admin")
f, _ = filter.AtoFilter("NOT status EQ banned")

// 括號控制優先順序
f, _ = filter.AtoFilter("(age GT 20 AND age LT 30) OR score >= 90")

// 動態組合查詢
f, _ = filter.AtoFilter(
    fmt.Sprintf("tool_name EQ %s AND symptom LIKE *%s*", toolName, keyword),
)
results = s.Query(f, limit)
```

### REPL 互動模式

```bash
go run cmd/test/main.go
```

```
toriidb[0]> SET user:1 {"name":"Alice","age":25}
OK
toriidb[0]> GET user:1
{"name":"Alice","age":25}
toriidb[0]> QUERY age GT 20 AND name LIKE Ali*
1) user:1: {"name":"Alice","age":25}
toriidb[0]> SELECT 3
OK
toriidb[3]>
```
