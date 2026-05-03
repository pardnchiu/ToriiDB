# ToriiDB - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25 或更高版本
- 具備讀寫權限的本地目錄（預設 `./temp`）
- **選用**：`.env` 中設定 `OPENAI_API_KEY`（僅 `SET ... VECTOR` / `VSEARCH` / `VSIM` / `VGET` 會用到）

核心儲存無外部服務依賴。向量功能在首次使用時才初始化 OpenAI client，未設 key 時自動 no-op。

## 安裝

### 使用 go get

```bash
go get github.com/agenvoy/toriidb
```

### 向量功能的 .env

```bash
# .env
OPENAI_API_KEY=sk-...
```

### 從原始碼建置 REPL

```bash
git clone https://github.com/agenvoy/toriidb.git
cd toriidb
go build -o toriidb ./cmd/test
./toriidb
```

### 直接執行 REPL

```bash
go run ./cmd/test
```

## 使用方式

### 初始化 Store

```go
package main

import (
    "fmt"
    "log"

    "github.com/agenvoy/toriidb/core/store"
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

### 自訂儲存路徑與 Session

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

### TTL 與過期

```go
exp := time.Now().Unix() + 60
if err := s.Set("token:abc", "payload", store.SetDefault, &exp); err != nil {
    log.Fatal(err)
}

ttl := s.TTL("token:abc")          // 剩餘秒數
_ = s.Expire("token:abc", 300)      // 300 秒後過期
_ = s.Persist("token:abc")          // 移除過期
```

### JSON 巢狀欄位

```go
_ = s.Set("user:1", `{"name":"Alice","addr":{"city":"Taipei"}}`, store.SetDefault, nil)

val, _ := s.GetField("user:1", []string{"addr", "city"})
_ = s.SetField("user:1", []string{"addr", "zip"}, "100", store.SetDefault, nil)
_ = s.DelField("user:1", []string{"addr", "zip"})

_, _ = s.IncrField("user:1", []string{"age"}, 1)
```

### Find 全域值搜尋

```go
import "github.com/agenvoy/toriidb/core/store/filter"

keys := s.Find(filter.EqualTo, "Alice", 0)
keys = s.Find(filter.GreaterThan, "50", 10)
keys = s.Find(filter.Like, "*error*", 0)
```

### Query 結構體組合

```go
import "github.com/agenvoy/toriidb/core/store/filter"

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

### Query 字串表達式

```go
f, err := filter.AtoFilter("(age GT 20 AND age LT 30) OR score >= 90")
if err != nil {
    log.Fatal(err)
}
results := s.Query(f, 0)
```

字串表達式適合拼接外部輸入：

```go
f, _ = filter.AtoFilter(
    fmt.Sprintf("tool EQ %s AND symptom LIKE *%s*", tool, keyword),
)
```

### 語意向量搜尋

```go
ctx := context.Background()

_ = s.SetVector(ctx, "doc:1", "How to cook pasta", store.SetDefault, nil)
_ = s.SetVector(ctx, "doc:2", "Building a web server in Go", store.SetDefault, nil)
_ = s.SetVector(ctx, "doc:3", "Italian dinner recipes", store.SetDefault, nil)

keys, err := s.VSearch(ctx, "cooking recipe", "doc:*", 2)
if err != nil {
    log.Fatal(err)
}
// keys == []string{"doc:3", "doc:1"} — 依 cosine 降序回傳 top-K

score, err := s.VSim("doc:1", "doc:3")
// score ~= 0.82（cosine 相似度，範圍 -1..1）

vec, ok := s.VGet("doc:1") // 深拷貝 []float32，外部修改不影響 Entry.Vector
```

### REPL 互動

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

## API 參考

### 核心類型

| 類型 | 說明 |
|------|------|
| `Store` | 頂層 handle，擁有 16 個 DB 與背景過期清理 goroutine |
| `Session` | 由 `Store.Session()` 衍生，共享資料但擁有獨立 db index |
| `Entry` | 單筆記錄，`value` 與 `parsed` 為私有欄位，僅能透過方法讀寫 |
| `ValueType` | 寫入時自動判定的型別列舉（JSON / String / Int / Float / Bool / Date） |
| `SetFlag` | `SetDefault` / `SetNX` / `SetXX` |

### Store 生命週期

| 函式 | 簽章 | 說明 |
|------|------|------|
| `New` | `func New(path ...string) (*Store, error)` | 建立 Store；無參數使用 `./temp`，傳一個字串則使用指定目錄 |
| `(*Store).Close` | `func (s *Store) Close() error` | 停止 cleanTimer 並並行壓縮所有活躍 DB 的 AOF |
| `(*Store).Session` | `func (s *Store) Session() *Session` | 衍生獨立 db index 的 Session |

### DB 切換

| 方法 | 簽章 | 說明 |
|------|------|------|
| `Select` | `func (c *core) Select(index int) error` | 切換 DB，合法範圍 0-15 |
| `Current` | `func (c *core) Current() int` | 目前 db index |
| `DB` | `func (c *core) DB() *db` | 取得目前 db，首次呼叫會觸發 lazy AOF replay |

### 讀寫

| 方法 | 簽章 | 說明 |
|------|------|------|
| `Set` | `func (c *core) Set(key, value string, flag SetFlag, expireAt *int64) error` | 寫入；JSON 會快取 parsed |
| `Get` | `func (c *core) Get(key string) (*Entry, bool)` | 讀取，過期則 lazy delete |
| `SetField` | `func (c *core) SetField(key string, subKeys []string, value string, flag SetFlag, expireAt *int64) error` | 以 dot-notation 寫入巢狀欄位 |
| `GetField` | 讀取巢狀欄位原始字串 | |
| `Del` | `func (c *core) Del(keys ...string) int` | 批次刪除，回傳實際刪除數 |
| `DelField` | `func (c *core) DelField(key string, subKeys []string) error` | 刪除巢狀欄位 |
| `Exist` / `ExistField` | 回傳 `(integer) 0/1` 字串 | |
| `Type` / `TypeField` | 回傳型別字串（如 `json`） | |
| `Incr` | `func (c *core) Incr(key string, delta float64) (float64, error)` | 數值遞增 |
| `IncrField` | `func (c *core) IncrField(key string, subKeys []string, delta float64) (float64, error)` | 巢狀欄位遞增 |
| `Keys` | `func (c *core) Keys(pattern string) []string` | Glob 匹配 |

### 過期

| 方法 | 簽章 | 說明 |
|------|------|------|
| `TTL` | `func (c *core) TTL(key string) int64` | 剩餘秒數，`-1` 無過期、`-2` 不存在 |
| `Expire` | `func (c *core) Expire(key string, seconds int64) error` | 指定秒數後過期 |
| `ExpireAt` | `func (c *core) ExpireAt(key string, ts int64) error` | 指定 Unix 時間戳 |
| `Persist` | `func (c *core) Persist(key string) error` | 移除過期設定 |

### 查詢

| 方法 | 簽章 | 說明 |
|------|------|------|
| `Find` | `func (c *core) Find(op filter.Operator, value string, limit int) []string` | 全域值比對，逾 1024 筆自動並發切塊 |
| `Query` | `func (c *core) Query(f filter.Filter, limit int) []string` | JSON 欄位條件查詢，接受任何 `filter.Filter` |
| `Exec` | `func (c *core) Exec(input string) string` | REPL 命令路由 |

### 向量

| 方法 | 簽章 | 說明 |
|------|------|------|
| `SetVector` | `func (c *core) SetVector(ctx context.Context, key, value string, flag SetFlag, expireAt *int64) error` | 主 key 同步寫入，背景 goroutine 非同步補 embedding（`Close()` 透過 `sync.WaitGroup` 等待排空） |
| `VSearch` | `func (c *core) VSearch(ctx context.Context, text, pattern string, k int) ([]string, error)` | Top-K cosine 搜尋；`k <= 0` 預設為 10，`pattern` 以 glob 過濾 key，自動跳過 `__torii:*` 內部 key |
| `VSim` | `func (c *core) VSim(key1, key2 string) (float64, error)` | 兩個 key 的向量 cosine 相似度；任一向量缺失回 `errVectorMissing`、維度不符回 `errVectorMismatch` |
| `VGet` | `func (c *core) VGet(key string) ([]float32, bool)` | 以深拷貝回傳儲存的向量，避免外部修改 `Entry.Vector` |

### filter 套件

| 類型 | 說明 |
|------|------|
| `Filter` | 核心介面 `Match(obj any) bool` |
| `Cond` | `{Field, Op, Value}` 基本條件 |
| `And` / `Or` / `Not` | 邏輯組合器 |
| `EQ` / `NE` / `GT` / `GTE` / `GE` / `LT` / `LTE` / `LE` / `LIKE` | 語法糖條件 |
| `Operator` | 運算子列舉（`EqualTo`, `NotEqualTo`, `GreaterThan`, `GreaterThanOrEqualTo`, `LessThan`, `LessThanOrEqualTo`, `Like`） |
| `AtoFilter` | `func AtoFilter(str string) (Filter, error)` — 字串表達式解析 |
| `AtoOperation` | `func AtoOperation(s string) (Operator, bool)` — 字串轉運算子 |
| `Match` | `func Match(stored string, op Operator, target string) bool` — 原始值比對 |

### REPL 指令

| 指令 | 語法 | 說明 |
|------|------|------|
| `GET` | `GET <key[.field...]>` | 讀取 key 或巢狀欄位 |
| `SET` | `SET <key> <value> [NX\|XX] [<seconds>] [VECTOR]` | 寫入；尾端整數為 TTL 秒數，尾端 `VECTOR` 附掛 OpenAI embedding |
| `DEL` | `DEL <key> [key2...]` 或 `DEL <key.field>` | 批次刪除或單一欄位刪除 |
| `EXIST` | `EXIST <key[.field...]>` | 回傳 `(integer) 0/1` |
| `TYPE` | `TYPE <key[.field...]>` | 回傳型別字串 |
| `INCR` | `INCR <key[.field...]> [delta]` | 預設 delta = 1 |
| `TTL` / `EXPIRE` / `EXPIREAT` / `PERSIST` | 過期控制 | |
| `KEYS` | `KEYS <pattern>` | glob 匹配 |
| `FIND` | `FIND <op> <value> [LIMIT <n>]` | 全域值搜尋 |
| `QUERY` | `QUERY <expression> [LIMIT <n>]` | 中綴表達式查詢 |
| `VSEARCH` | `VSEARCH <text> [MATCH <pattern>] [LIMIT <n>]` | Top-K cosine 搜尋；`MATCH` / `LIMIT` 順序可交換；預設 `LIMIT 10` |
| `VSIM` | `VSIM <key1> <key2>` | 兩個 key 的向量 cosine 相似度；任一缺向量回 `(nil)` |
| `VGET` | `VGET <key>` | 以 JSON 陣列格式回傳儲存向量（除錯用） |
| `SELECT` | `SELECT <0-15>` | 切換 DB |
| `exit` / `quit` | 離開 REPL | |

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
