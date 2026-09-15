# toriidb - 技術文件

最後更新：2026-09-16

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25 以上版本
- 支援 unix domain socket 的 Unix-like 系統（daemon 監聽於 `/tmp`）
- 可寫入的資料目錄
- 選用：`OPENAI_API_KEY`，僅向量操作需要

## 安裝

### 使用 go get

```bash
go get github.com/pardnchiu/toriidb
```

### 從原始碼建置

```bash
git clone https://github.com/pardnchiu/toriidb.git
cd toriidb
go build ./...
```

### Make 目標

| 目標 | 動作 |
|---|---|
| `make test` | 以 `./temp` 啟動 REPL |
| `make unit` | `go test ./core/... -count=1 -cover` |
| `make embed` | 執行 OpenAI embedding 整合測試 |

## 設定

### 環境變數

| 變數 | 必填 | 預設 | 說明 |
|---|---|---|---|
| `OPENAI_API_KEY` | 否 | 無 | 啟用 embedding；未提供時向量操作回傳 `ErrNoEmbedder` |
| `TORIIDB_EMBED_DIM` | 否 | `256` | Embedding 維度；`<= 0` 時退回 `256` |

### API Key 查找順序

1. `store.New(dir, apiKey)` / `daemon.New(dir, apiKey)` 的選用 `apiKey` 參數
2. `OPENAI_API_KEY` 環境變數
3. 作業系統 keychain 中 service `ToriiDB` 下的 `OPENAI_API_KEY`

### 設定檔

Go 程式不會載入 `.env`。`Makefile` 在存在本地 `.env` 時會 `include` 並 `export`：

```bash
cat <<'EOF' > .env
OPENAI_API_KEY=your_api_key
TORIIDB_EMBED_DIM=256
EOF
make test
```

## 使用方式

### Basic

透過 socket daemon 開啟資料目錄。同目錄第一個呼叫端成為 server，其後皆為 client，呼叫的方法相同；client 模式下 `Start` 不做任何事：

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

單一 process 獨佔目錄時，直接使用內嵌 store：

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

REPL：

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

JSON 欄位與查詢運算式。`SetField` 只更新巢狀欄位，不重寫整份文件；`Query` 每一列為 `key: rawValue`：

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

透過 daemon 做向量搜尋、分頁與伺服器端篩選。向量於背景附加，`SetVector` 回傳後新寫入的鍵稍後才會出現在 `VSearch` 結果中：

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

## API 參考

### daemon

| 函式 / 方法 | 說明 |
|---|---|
| `New(dir string, apiKey ...string) (*Daemon, error)` | 解析目錄並推導 socket；socket 存活為 `ModeClient`，否則開啟 store 成為 `ModeServer` |
| `(*Daemon) Start() error` | 不阻塞地開始監聽；另一個 daemon 已在服務時回傳 `ErrAlreadyRunning`；client 模式不做事 |
| `(*Daemon) Stop() error` | 關閉 listener 與所有連線並等待處理 goroutine 結束 |
| `(*Daemon) Close() error` | Server：`Stop` 後關閉 store；client：關閉閒置連線 |
| `(*Daemon) Mode() Mode` / `Socket() string` / `Store() *store.Store` | 模式、socket 路徑、底層 store（client 模式為 `nil`） |
| `Get(ctx, db, key) (*Record, error)` | 讀取一筆；不存在回 `ErrNotFound` |
| `Entries(ctx, db, pattern, *ScanOption) ([]Record, error)` | 伺服器端以 `Contains`／`After`／`Limit`（預設 100，取最後 n 筆）篩選 |
| `Set(ctx, db, key, content any, expireAt *int64) error` | Upsert；字串存為原文，其他值存為壓縮 JSON |
| `SetVector(ctx, db, key, content any, expireAt *int64) error` | Upsert 並於背景附加 embedding |
| `Del(ctx, db, keys ...string) (int, error)` | 回傳刪除數量 |
| `TTL(ctx, db, key) (int64, error)` | 剩餘秒數，永久為 `-1` |
| `Expire(ctx, db, ttl int64, keys ...string) (int, error)` | 回傳成功套用的鍵數；`ttl` 必須 `> 0` |
| `Keys(ctx, db, pattern, limit, page) (*ListResult, error)` | `limit <= 0` 回傳全部鍵 |
| `VSearch(ctx, db, text, pattern, limit) ([]string, error)` | 依相似度排序；`limit <= 0` 為 `100` |
| `HasEmbedder(ctx) (bool, error)` | Server 端是否有 embedder |
| `(*Record) Value() (string, error)` | 取回儲存的原文；不要自行 `string(record.Content)` |

錯誤：`ErrAlreadyRunning`、`ErrNotFound`、`ErrBadRequest`、`ErrNoEmbedder`，以 `errors.Is` 判斷。

### store

`Store` 與 `Session` 內嵌同一組方法；併發呼叫端各自使用 `Store.Session()`。

| 函式 / 方法 | 說明 |
|---|---|
| `New(dir string, apiKey ...string) (*Store, error)` | `dir` 必填；16 個資料庫於首次使用時載入 |
| `(*Store) Session() *Session` | 共用資料、各自持有選定的資料庫索引 |
| `(*Store) Close() error` | 冪等；等待向量工作、壓縮並回收舊世代 |
| `Select(index int) error` / `Current() int` | 切換／取得資料庫（`0-15`） |
| `Exec(input string) string` | 執行 REPL 指令文字 |
| `Set` / `SetField` / `SetVector` | 寫入值、巢狀欄位、帶向量的值；`SetFlag` 為 `SetDefault`／`SetNX`／`SetXX` |
| `Get(key) (*Entry, bool)` / `GetField(key, subKeys) (string, bool)` | 讀取 entry 複本或巢狀欄位 |
| `Incr(key, delta)` / `IncrField(key, subKeys, delta)` | 數值遞增 |
| `Del(keys...) int` / `DelField(key, subKeys) error` | 刪除鍵或欄位 |
| `TTL` / `Expire` / `ExpireAt` / `Persist` | 過期管理；`TTL` 不存在為 `-2` |
| `Keys(pattern) []string` | `path.Match` glob，字典序 |
| `Find(op, value, limit) []string` | 比較整個原始值，最新優先 |
| `Query(f filter.Filter, limit) []string` | 篩選 JSON 文件，回傳 `key: value` |
| `VSearch(ctx, text, pattern, k)` / `VSim(key1, key2)` / `VGet(key)` | 向量搜尋、相似度、取得向量；`k <= 0` 為 `10` |
| `HasEmbedder() bool` | 是否已建立 embedder |

### filter

| 名稱 | 說明 |
|---|---|
| `AtoFilter(str string) (Filter, error)` | 解析 `field op value`、`AND`／`OR`／`NOT` 與括號 |
| `AtoOperation(s string) (Operator, bool)` | `EQ`/`=`、`NE`/`!=`、`GT`/`>`、`GTE`/`GE`/`>=`、`LT`/`<`、`LTE`/`LE`/`<=`、`LIKE` |
| `And` / `Or` / `Not` / `Cond` / `EQ` ... `LIKE` | 可組合的程式化篩選型別 |

### REPL 指令

| 指令 | 語法 |
|---|---|
| `GET` / `EXIST` / `TYPE` | `GET <key>`（`key.field` 讀巢狀欄位） |
| `SET` | `SET <key> <value> [NX\|XX] [seconds] [VECTOR]` |
| `DEL` | `DEL <key> [key2] ...` |
| `INCR` | `INCR <key> [delta]` |
| `TTL` / `EXPIRE` / `EXPIREAT` / `PERSIST` | `EXPIRE <key> <seconds>`、`EXPIREAT <key> <timestamp>` |
| `KEYS` | `KEYS <pattern>` |
| `FIND` | `FIND <op> <value> [LIMIT <n>]` |
| `QUERY` | `QUERY <expression> [LIMIT <n>]` |
| `VSEARCH` | `VSEARCH <text> [MATCH <pattern>] [LIMIT <n>]` |
| `VSIM` / `VGET` | `VSIM <key1> <key2>`、`VGET <key>` |
| `SELECT` | `SELECT <db>`（`0-15`） |

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
