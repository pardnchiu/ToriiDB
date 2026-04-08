# ToriiDB

> 專案 [go-redis-fallback](https://github.com/pardnchiu/go-redis-fallback) 的延伸<br>
> 聚焦在以 JSON 為核心的資料庫，實現 CRUD、Redis 快取與過期淘汰，以及 MongoDB 的基本查找<br>
> 只以標準庫實現，目標是取代 SQLite 作為 Golang 中小型專案的資料庫基礎

## 已完成功能

### 2026-04-08

- `KEYS` glob 模式匹配
- `GET`/`SET`/`DEL` 支援 dot-notation 巢狀欄位存取
- `INCR` 數值遞增（獨立 key 與巢狀 JSON 欄位）
- `FIND` 全域值搜尋（EQ / GT / GE / LT / LE / LIKE）
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
