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
| `TRAVIS_TOKEN` | Travis CI API 權杖（`sync` 與 `repo sync` 需要） |

## 指令參考

### `sync`

```
devpulse sync
```

對資料庫中的每一個 repo 依序執行 `repo sync`。這是設計給 cron / CI 排程的入口：先用 `repo add` 註冊 repo，之後就以任意週期排 `devpulse sync`。

- **Disabled repo 會被跳過**（會印一行 `skipped <owner/name> (disabled)`）；`disabled = true` 代表 GitHub 端已 archive 或停用該 repo，繼續同步會浪費 API 配額且幾乎必定失敗。
- **單一 repo 失敗不會中斷迴圈。** 失敗會立即印出（`failed <owner/name>: <err>`）、被記錄下來，下一個 repo 繼續執行。所有 repo 跑完後會印彙整（`sync: synced=N skipped=M failed=K`），並列出所有失敗 repo 與錯誤訊息方便 grep。
- **任何 repo 失敗時，整體 exit code 非零**，cron / CI 可直接用回傳碼判斷整批健康度。
- **循序執行，不並行。** GitHub 與 Travis 都對單一 token 做 rate limit，並行只會讓配額集中爆掉、沒有明顯吞吐量收益。若要對單一 repo 即時同步，直接用 `repo sync`。

`GITHUB_TOKEN` 與 `TRAVIS_TOKEN` 都是必要參數，且會在開啟資料庫之前先檢查（fail-fast）。

**引數**

無。

**輸出**

```
Synced MilesChou/devpulse pull requests: written=7
Synced MilesChou/devpulse ci builds: written=42
skipped acme/legacy (disabled)
failed acme/broken: sync pull requests: github: 404 Not Found

sync: synced=1 skipped=1 failed=1
failures:
  acme/broken: sync pull requests: github: 404 Not Found
```

**範例**

```sh
devpulse sync
```

---

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

### `repo config set` / `repo config get`

```
devpulse repo config set <owner/name> <key> <value>
devpulse repo config get <owner/name> [key]
```

讀取或設定指定儲存庫的操作設定（per-repo operator settings）。設定屬於 operator 擁有，**不會**被 `repo sync` 覆寫——一旦寫入即生效，直到下次 `config set` 才會變動。

**可用設定**

| 設定名稱 | 型別 | 說明 |
|---|---|---|
| `pr-start` | 整數（>= 1） | PR 同步起點（floor）——`devpulse repo sync` 在 by-number 模式下會從這個 PR number 開始往上掃描。預設 `1`（抓全部歷史）。當早期 PR 未接 CI、不具觀測價值時將起點調高即可節省 GitHub API 配額。 |

該儲存庫必須已透過 `devpulse repo add` 註冊。`repo config get` 若不帶 `key` 引數，會列出所有已知設定值。

**範例**

```sh
# 跳過 PR #1 到 #499（例如尚未接 CI 的早期歷史），從 #500 開始抓
devpulse repo config set MilesChou/devpulse pr-start 500

# 讀回單一設定
devpulse repo config get MilesChou/devpulse pr-start
# → 500

# 或一次列出所有設定
devpulse repo config get MilesChou/devpulse
# → pr-start=500
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

> 首次執行最耗時：PR 同步會從 `pr_sync_start_number`（預設 1）開始往上、逐個 PR number 抓 detail + reviews，跑到 GitHub 當前最大 PR number 為止；build 同步則會走完整個 Travis 歷史，無頁數上限。後續執行為增量——PR 從 `MAX(number) + 1` 接著抓、build 用 `(repo_id, external_id)` 去重、author backfill 只會處理 author 仍為 NULL 的 commit SHA。

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

啟動長期執行的背景工作程序。Worker 會持續輪詢資料庫中的待執行工作（例如由 `sync` / `repo sync` 排入佇列的補充工作）並加以處理。按 `Ctrl-C`（`SIGINT`）或傳送 `SIGTERM` 以停止。

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

# 2'. （選用）跳過尚未接 CI 的早期歷史，將 PR 同步起點設高。
#     未設定時，首次同步會從 PR #1 開始往上抓。
devpulse repo config set MilesChou/devpulse pr-start 500

# 3. 同步該 repo 的所有 Pull Request（含 enrichment）與 CI 建置記錄
devpulse repo sync MilesChou/devpulse

# 3'. ...或者，當已經註冊多個 repo 時，一次同步全部——這也是 cron / CI
#     排程要用的指令。
devpulse sync

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
