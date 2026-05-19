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
| `TRAVIS_TOKEN` | Travis CI API 權杖（僅 `build fetch` 需要） |

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

### `build fetch`

```
devpulse build fetch <owner/name> <YYYY-MM>
```

從 Travis CI 取得指定儲存庫與月份的 CI 建置記錄，並寫入資料庫。需要設定 `TRAVIS_TOKEN`。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |
| `YYYY-MM` | 年月格式，例如 `2026-05` |

**輸出**

```
Fetched MilesChou/devpulse builds for 2026-05: written=42
```

**範例**

```sh
devpulse build fetch MilesChou/devpulse 2026-05
```

---

### `pr fetch`

```
devpulse pr fetch <owner/name> <YYYY-MM>
```

從 GitHub 取得指定儲存庫與月份的 Pull Request（含審查意見與提交詳情），寫入資料庫後自動執行補充（enrich）。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |
| `YYYY-MM` | 年月格式，例如 `2026-05` |

**輸出**

```
Fetched MilesChou/devpulse PRs for 2026-05: written=7
```

**範例**

```sh
devpulse pr fetch MilesChou/devpulse 2026-05
```

---

### `pr enrich`

```
devpulse pr enrich <owner/name> <number>
```

重新取得資料庫中已存在的單一 Pull Request 的詳細資料與審查記錄，並寫入補充更新。適用於不需要重抓整個月份、只需刷新特定 PR 的情境。

該 PR 必須已存在於資料庫中。若尚未存在，請先執行 `devpulse pr fetch`。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |
| `number` | Pull Request 編號，例如 `42` |

**輸出**

```
Enriched MilesChou/devpulse#42
```

**範例**

```sh
devpulse pr enrich MilesChou/devpulse 42
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

啟動長期執行的背景工作程序。Worker 會持續輪詢資料庫中的待執行工作（例如由 `pr fetch` 排入佇列的補充工作）並加以處理。按 `Ctrl-C`（`SIGINT`）或傳送 `SIGTERM` 以停止。

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

# 3. 補抓某個月份的 CI 建置記錄與 Pull Request
devpulse build fetch MilesChou/devpulse 2026-05
devpulse pr fetch MilesChou/devpulse 2026-05

# 4. （選用）刷新單一 PR
devpulse pr enrich MilesChou/devpulse 42

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
make run ARGS="pr fetch MilesChou/devpulse 2026-05"
```
