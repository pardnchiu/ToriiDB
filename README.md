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
