# DevPulse CLI 指令說明

DevPulse 採用**名詞-動詞**的指令結構（風格類似 `gh` 與 `jira-cli`）：

```
devpulse <名詞> <動詞> [引數] [旗標]
```

## 前置需求

除 `migrate` 以外的所有指令，皆需要設定以下環境變數（請參考 `.env.example`）：

| 變數 | 說明 |
|---|---|
| `DEVPULSE_DSN` | 資料庫連線字串（支援 PostgreSQL、MySQL、SQLite 或 `memory`） |
| `GITHUB_TOKEN` | GitHub 個人存取權杖（需要 `repo` + `read:user` 範圍） |
| `TRAVIS_TOKEN` | Travis CI API 權杖（`repo sync` 需要） |

## 指令參考

### `repo add`

```
devpulse repo add <owner/name>
```

將 GitHub 儲存庫註冊至 DevPulse 資料庫。若儲存庫已存在，則直接回傳既有記錄（冪等操作）。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱，例如 `MilesChou/devpulse` |

**輸出**

```
MilesChou/devpulse (id=01J5HQ...)
```

**範例**

```sh
devpulse repo add MilesChou/devpulse
```

---

### `repo sync`

```
devpulse repo sync <owner/name>
```

依序執行兩個步驟同步指定儲存庫：

1. **Pull Request**：從 GitHub 抓取所有 PR（含審查與 commit 細節），寫入資料庫並執行 enrichment。
2. **CI builds**：從 Travis CI 抓取所有建置記錄並寫入資料庫。

PR 步驟先跑；若失敗則跳過 build 步驟並以非零狀態結束。需要同時設定 `GITHUB_TOKEN` 與 `TRAVIS_TOKEN`。

> 首次執行最耗時：PR 同步會分頁打完整個 PR 歷史（吃掉相當比例的 GitHub REST 與 GraphQL 配額），build 同步則會走完整個 Travis 歷史（上限 100 頁 × 100 build）。後續執行為增量——upsert 會去重、author backfill 只會處理 author 仍為 NULL 的 commit SHA、PR 也會在進到下一頁前完成 upsert 與 enrichment，所以中斷的執行仍會保留已處理的進度。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |

**輸出**

```
Synced MilesChou/devpulse PRs: written=7
Synced MilesChou/devpulse builds: written=42
```

**範例**

```sh
devpulse repo sync MilesChou/devpulse
```

---

### `pr sync`

```
devpulse pr sync <owner/name> <number>
```

重新取得資料庫中已存在的單一 Pull Request 的詳細資料與審查記錄，並寫入補充更新。適用於不需要重新同步整個儲存庫、只需刷新特定 PR 的情境。

該 PR 必須已存在於資料庫中。若尚未存在，請先執行 `devpulse repo sync`。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |
| `number` | Pull Request 編號，例如 `42` |

**輸出**

```
Synced MilesChou/devpulse#42
```

**範例**

```sh
devpulse pr sync MilesChou/devpulse 42
```

---

### `migrate up`

```
devpulse migrate up
```

套用所有待執行的資料庫結構遷移。可重複執行——已套用的遷移會自動略過。

**輸出**

```
migrations up: ok
```

---

### `migrate down`

```
devpulse migrate down
```

回滾最近一次套用的遷移（一次一步）。

**輸出**

```
migrations down: ok
```

---

### `migrate status`

```
devpulse migrate status
```

顯示已套用的遷移版本清單。

**輸出**

```
applied 3 migrations:
  1
  2
  3
```

---

### `worker`

```
devpulse worker [--poll <時間間隔>] [--lease <時間間隔>]
```

啟動長期執行的背景工作程序。Worker 會持續輪詢資料庫中的待執行工作（例如由 `repo sync` 排入佇列的補充工作）並加以處理。按 `Ctrl-C`（`SIGINT`）或傳送 `SIGTERM` 以停止。

**旗標**

| 旗標 | 預設值 | 說明 |
|---|---|---|
| `--poll` | `5s` | 佇列為空時的輪詢間隔 |
| `--lease` | `60s` | 工作租約時間，超時後卡住的工作將重新排入佇列 |

**輸出**

```
worker started; press Ctrl-C to stop
worker stopped
```

**範例**

```sh
# 開發時縮短輪詢間隔
devpulse worker --poll 2s
```

---

### `serve`

```
devpulse serve
```

**預留位置——v1 尚未實作。** 印出提示訊息後結束。HTTP API 介面規劃於未來版本提供。

---

## 典型工作流程

```sh
# 1. 套用資料庫結構遷移
devpulse migrate up

# 2. 註冊目標儲存庫
devpulse repo add MilesChou/devpulse

# 3. 同步所有 Pull Request（含 enrichment）與 CI 建置記錄
devpulse repo sync MilesChou/devpulse

# 4. （選用）刷新單一 PR
devpulse pr sync MilesChou/devpulse 42

# 5. （選用）啟動背景 Worker 處理非同步補充工作
devpulse worker
```

## 開發捷徑（Makefile）

儲存庫根目錄的 `Makefile` 提供常用的便利目標。執行 `make help` 可查看完整清單。

| 目標 | 說明 |
|---|---|
| `make build` | 將二進位檔編譯至 `./bin/devpulse` |
| `make run ARGS="..."` | 編譯、載入 `.env`，再執行 `./bin/devpulse <ARGS>` |
| `make test` | 執行單元測試 |
| `make test-race` | 以 `-race` 旗標執行單元測試 |
| `make test-integration` | 執行整合測試（需要 Docker） |
| `make lint` | 執行 `go vet` + `gofmt` 檢查 |
| `make tidy` | 執行 `go mod tidy` |
| `make clean` | 刪除 `./bin/` |

**範例**

```sh
make run ARGS="repo sync MilesChou/devpulse"
```
