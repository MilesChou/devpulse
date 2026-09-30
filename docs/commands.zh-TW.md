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
| `TRAVIS_TOKEN` | *（選填）* Travis CI API 權杖。設定後 `sync` / `repo sync` 除了 GitHub Actions runs 之外也會抓取 Travis builds |

選用的 HTTP 回應快取（預設關閉）：

| 變數 | 說明 |
|---|---|
| `CACHE_ENABLED` | 設為 `true` 時將成功的 API 回應快取到磁碟 |
| `CACHE_DIR` | 快取目錄（預設：`os.UserCacheDir()/devpulse`） |
| `CACHE_TTL` | 新鮮度時間窗（預設 `24h`）。超過 TTL 的快取會以 ETag / If-Modified-Since 重新驗證——304 不消耗 GitHub rate limit。**`CACHE_TTL=0` 為 replay 模式**：快取永不過期，可在不連網的情況下重建資料庫——但上游的任何變動都看不到。例行同步切勿設為 `0` |

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

`GITHUB_TOKEN` 為必要參數，且會在開啟資料庫之前先檢查（fail-fast）。`TRAVIS_TOKEN` 為選填——未設定時只同步 GitHub Actions builds。

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
| `incident-label` | 字串（不可空白） | 標記事故的 issue label，預設 `incident`。每次同步都會把帶這個 label 的 issue 全部鏡像到資料庫，作為 DORA 恢復時間的資料來源。改動後下次同步生效。 |
| `hotfix-label` | 字串（不可空白） | 標記 hotfix 的 PR label，預設 `hotfix`，計入 DORA 變更失敗率。在 `metrics` 執行時才套用，改動後不需要重新同步。 |

Label 比對不分大小寫。

該儲存庫必須已透過 `devpulse repo add` 註冊。`repo config get` 若不帶 `key` 引數，會列出所有已知設定值。

**範例**

```sh
# 跳過 PR #1 到 #499（例如尚未接 CI 的早期歷史），從 #500 開始抓
devpulse repo config set MilesChou/devpulse pr-start 500

# 讀回單一設定
devpulse repo config get MilesChou/devpulse pr-start
# → 500

# 換成團隊自己的 DORA label
devpulse repo config set MilesChou/devpulse incident-label sev-1
devpulse repo config set MilesChou/devpulse hotfix-label urgent-fix

# 或一次列出所有設定
devpulse repo config get MilesChou/devpulse
# → pr-start=500
# → incident-label=sev-1
# → hotfix-label=urgent-fix
```

---

### `repo sync`

```
devpulse repo sync <owner/name>
```

依序執行五個步驟同步指定儲存庫：

1. **重抓有變動的 PR**：重抓資料庫中、GitHub 回報自上次重抓後有更新的 PR，以及所有狀態為 open 的 PR，把首次同步之後才發生的 merge、close、重新開啟、改標題或改 label 記錄下來（merge 就是 DORA 的部署）。水位是已看到的 PR 中最新的 `updated_at`，取自 GitHub 列表本身，所以列表就算來自 cache 也不會跳過更新。抓回來的 PR 比列表上的舊（來自 cache 的舊資料）時不會寫入。單一 PR 重抓失敗只會記錄 log，水位不前進，下次同步會再列到它。open 的 PR 每次都重抓，這樣 cache 裡的舊資料（例如開啟 `CACHE_ENABLED=true` 重建資料庫時寫入的）會在 cache 過期後自動更正。
2. **Pull Request**：從 GitHub 抓取所有新的 PR（detail、reviews；已 merge 的 PR 另外抓最早的 commit 時間），寫入資料庫並執行 enrichment。抓最早的 commit 時間失敗不會讓該 PR 失敗，由步驟 3 重試。早於 1971 年的 commit author 時間（時鐘錯誤）會被忽略，晚於 merge 的時間則以 merge 時間儲存。
3. **補齊 PR 資料**：重抓資料庫中缺少 DORA 資料的 PR，也就是沒有 base branch（在 DORA 欄位加入前寫入的資料列），或已 merge 但沒有最早 commit 時間的 PR。在支援 DORA 之前就同步過的資料庫，靠這一步補齊歷史資料。依 PR number 由小到大處理，遇到第一個失敗就印出警告並停止，下次同步從該處繼續，不會讓指令失敗。
4. **CI builds**：從每一個已註冊的 CI provider（GitHub Actions 必有；設定 `TRAVIS_TOKEN` 時加上 Travis CI）抓取建置記錄並寫入資料庫。GitHub Actions 回傳的 run 實際上不帶 PR 清單，所以接著會把每個由 PR 觸發的 build，對應到 head branch 與 build 的 branch 相同、且 build 開始時仍開啟中的 PR。同時符合兩個 PR 的 build（例如兩個同時開啟、branch 都叫 `main` 的 fork PR）不會被對應。
5. **事故（incidents）**：鏡像所有帶 repo `incident-label` 的 issue（包含 open 與 closed，排除 PR）。失敗時只印出警告，不會讓指令失敗。

步驟 1、2 或 4 失敗時會跳過後面的步驟，並以非零狀態結束。`GITHUB_TOKEN` 為必要；`TRAVIS_TOKEN` 為選填。

> 首次執行最耗時：PR 同步會從 `pr_sync_start_number`（預設 1）開始往上、逐個 PR number 抓 detail + reviews，跑到 GitHub 當前最大 PR number 為止；build 同步則會走完該 provider 的完整歷史，無頁數上限（該 provider 尚無任何資料列時才走這條 cold-start 路徑）。後續執行為增量——PR 從 `MAX(number) + 1` 接著抓，每個 CI provider 各自從**自己的** `MAX(started_at) - 6 小時` watermark 開始翻頁（per-provider 游標確保落後或新加入的 provider 不會繼承別人的進度；6 小時 overlap 用來吸收 retry build 與上次同步時還在執行中的 run——6 小時即 GitHub Actions 單一 job 的硬上限——已寫入的列由 `(repo_id, ci_provider, external_id)` unique constraint 靜默去重）、author backfill 只會處理 author 仍為 NULL 的 commit SHA。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |

**輸出**

```
Refreshed MilesChou/devpulse pull requests: 3
Synced MilesChou/devpulse pull requests: written=7
Completed MilesChou/devpulse pull request facts: 120
Synced MilesChou/devpulse ci builds: written=42
Synced MilesChou/devpulse incidents (label "incident"): 2
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

### `metrics`

```
devpulse metrics <owner/name> [--from YYYY-MM] [--to YYYY-MM]
```

印出指定 repo 在月份區間內的工程效率指標：CI 失敗率（僅計 PR builds）、每 PR 平均建置次數、PR lead time（平均 / p50 / p90）、review 等待時間、PR 大小分布、每日平均建置時長，以及四項 DORA 指標（見 [DORA 定義](#dora-定義)）。

`--from` 預設為當前月份；`--to` 為排除上界，預設為 `--from` 的下一個月。

**引數**

| 引數 | 說明 |
|---|---|
| `owner/name` | GitHub 儲存庫識別名稱 |

**旗標**

| 旗標 | 說明 |
|---|---|
| `--from` | 起始月份，包含（`YYYY-MM`；預設：當前月份） |
| `--to` | 結束月份，排除（`YYYY-MM`；預設：`--from` + 1 個月） |

**輸出**

```
Metrics for MilesChou/devpulse (2026-05)
────────────────────────────────────────
CI Failure Rate:        12.5% (3/24 PR builds)
Avg Builds per PR:      2.4
PR Lead Time:           avg 18.2h  p50 6.1h  p90 52.0h  (10 PRs)
Review Wait Time:       avg 3.4h (8 PRs)
PR Size Distribution:   XS:4  S:3  M:2  L:1

Daily Build Duration (avg seconds):
  2026-05-02: 74s (6 builds)
  2026-05-03: 81s (4 builds)

DORA (deployment = PR merged into default branch)
────────────────────────────────────────
Deployment Frequency:   10 deploys into main  (2.26/week, 6 deploy days)
Lead Time for Changes:  avg 20.5h  p50 8.0h  p90 60.2h  (10 deploys)
Change Failure Rate:    20.0% (2/10)  reverts=1 hotfixes=1 (label "hotfix")
Recovery Time:          avg 2.8h  p50 2.8h  p90 3.0h  (2 samples)  from reverts=1 incidents=1 (label "incident")
```

#### DORA 定義

每個事件都依它的結束時間歸入所屬的區間。

| 指標 | 定義 |
|---|---|
| Deployment Frequency（部署頻率） | merge 進 repo default branch 的 PR 數量，另外換算成每週次數，並列出有部署的 UTC 日數。查詢期間還沒結束時（預設的本月），每週次數只以目前已經過的天數計算。 |
| Lead Time for Changes（變更前置時間） | PR 中最早的 commit **author** 時間 → merge。author 時間在 rebase 後仍會保留。每個 PR 只看前 100 個 commit。負值（時鐘誤差）視為 0。 |
| Change Failure Rate（變更失敗率） | （revert + hotfix 部署數）÷ 部署數。**Revert**：標題以 "revert" 這個字開頭，後面接空白、`:`、`(`、`"`、`!` 或直接結束（`Revert "x"`、`revert: x`、`revert(api): x` 會算，`Revert-safe helper`、`revert/cleanup` 不算）。revert 一個 revert 等於把變更重新上線，所以 `Revert "Revert "x""` 不算 revert（巢狀 `Revert "…"` 的層數是奇數才算）。**Hotfix**：帶有 `hotfix-label`，或 head branch 以 `hotfix/` 開頭。同時符合兩者只算一次。沒有部署時顯示 `n/a`。 |
| Recovery Time（失敗部署恢復時間） | 資料來源有兩種：body 寫著 `Reverts owner/repo#N` 的 revert PR（GitHub revert 按鈕產生的格式），計算 #N merge 到 revert merge 的時間；以及帶 `incident-label` 的 issue，計算開啟到關閉的時間。尚未關閉的事故不列入。 |

限制：沒有經過 PR、直接推上 default branch 的 commit 看不到。hotfix PR 不會連結到它修復的那次部署。default branch 未知時，這個區塊會提示先執行 `devpulse repo refresh`。

> 在 DORA 欄位加入之前就同步的 PR 沒有 base branch，在 `repo sync` 補齊之前（補齊 PR 資料步驟）不會被算成部署。歷史資料多的 repo，每個 PR 要各抓一次 detail、reviews 和 commits，碰到 GitHub rate limit 時會分成好幾次同步才補完。若有設定 `CACHE_ENABLED=true`，detail 和 reviews 會用快取的回應，裡面已經有這些新欄位。

**範例**

```sh
devpulse metrics MilesChou/devpulse --from 2026-05 --to 2026-06
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

啟動唯讀的 JSON API，直到按下 `Ctrl-C`（`SIGINT`）或收到 `SIGTERM` 為止。[桌面 dashboard](../desktop/README.zh-TW.md) 是它的 client，其他能發 HTTP 請求的工具也都可以使用。

| 變數 | 預設值 | 說明 |
|---|---|---|
| `HTTP_ADDR` | `127.0.0.1:8080` | 監聽位址 |
| `DEVPULSE_API_TOKEN` | *（空）* | 每個 `/api/` 請求都必須帶上的 bearer token。當 `HTTP_ADDR` 不是 loopback 位址時**必填**，否則 `serve` 會拒絕啟動，因為 API 會公開所有追蹤中 repo 的資料 |

在容器裡執行時，預設的 loopback 位址從外部連不到：請設定 `HTTP_ADDR=0.0.0.0:8080`（因此也要設定 `DEVPULSE_API_TOKEN`），並把 port 對外開放。

**端點**

| 方法 + 路徑 | 認證 | 回傳 |
|---|---|---|
| `GET /healthz` | 不需要 | `{"status":"ok"}` |
| `GET /api/v1/repos` | bearer | `{"repos":[{id, full_name, owner, name, provider, description, default_branch, disabled}]}` |
| `GET /api/v1/repos/{owner}/{name}` | bearer | 單一 repo，格式同上 |
| `GET /api/v1/repos/{owner}/{name}/metrics?from=YYYY-MM&to=YYYY-MM` | bearer | 該時間範圍的報表（見下方） |
| `GET /api/v1/repos/{owner}/{name}/metrics/monthly?from=YYYY-MM&to=YYYY-MM` | bearer | `{repo, from, to, months:[報表, ...]}`，每個月一份報表，由舊到新 |

`from` / `to` 的行為和 `metrics` 指令的旗標完全相同：`from` 預設為當月（UTC），`to` 不包含在範圍內，預設為 `from` 加一個月。`metrics/monthly` 每次最多 36 個月（每個月都要各算一份報表）；`metrics` 沒有範圍上限。報表和 `devpulse metrics` 使用同一段程式計算，兩者的數字不會不一致。

**報表**

```json
{
  "repo": "MilesChou/devpulse",
  "from": "2026-05",
  "to": "2026-06",
  "build_failure": { "total": 3, "failed": 2, "rate": 0.6666666666666666 },
  "avg_builds_per_pr": 1.5,
  "pr_lead_time": { "count": 3, "avg_hours": 20, "p50_hours": 20, "p90_hours": 28 },
  "review_wait": { "count": 3, "avg_hours": 2 },
  "pr_size_distribution": [
    { "bucket": "XS", "count": 1 }, { "bucket": "S", "count": 1 },
    { "bucket": "M", "count": 0 }, { "bucket": "L", "count": 1 },
    { "bucket": "XL", "count": 0 }
  ],
  "daily_build_duration": [
    { "day": "2026-05-01", "avg_seconds": 90, "count": 2 }
  ],
  "dora": {
    "default_branch": "main",
    "hotfix_label": "hotfix",
    "incident_label": "incident",
    "deployments": 3,
    "per_week": 2.1,
    "deploy_days": 3,
    "lead_time": { "count": 3, "avg_hours": 22, "p50_hours": 22, "p90_hours": 30 },
    "reverts": 1,
    "hotfixes": 0,
    "change_failure_rate": 0.3333333333333333,
    "recovery": { "count": 2, "avg_hours": 36, "p50_hours": 36, "p90_hours": 61.6 },
    "recovery_from_reverts": 1,
    "recovery_from_incidents": 1
  }
}
```

`pr_size_distribution` 固定依尺寸由小到大列出五個 bucket；只有在部分 PR 沒有 bucket 時，才會多一筆 `unknown`。`dora` 依照 [DORA 定義](#dora-定義) 計算；repo 的 default branch 還不知道時為 `null`（請先執行 `repo refresh`），而沒有任何部署時，其中的 `change_failure_rate` 為 `null`。視窗還沒結束時，`per_week` 只計算到現在為止，和 `metrics` 指令相同。完整範例是 [`internal/http/testdata/`](../internal/http/testdata/) 裡的 golden 檔案，dashboard 的測試也解析同一批檔案。

**錯誤**格式為 JSON `{"error": "..."}`，狀態碼為 `400`（`from` / `to` 格式錯誤）、`401`（缺少 token 或 token 錯誤）、`404`（repo 未追蹤或路徑不存在）或 `500`（細節只記錄在 server log，不會回傳）。

**範例**

```sh
DEVPULSE_API_TOKEN=change-me devpulse serve
curl -H "Authorization: Bearer change-me" \
  "http://127.0.0.1:8080/api/v1/repos/MilesChou/devpulse/metrics?from=2026-05"
```

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

# 6. （選用）啟動指標 API，給桌面 dashboard 使用
DEVPULSE_API_TOKEN=change-me devpulse serve
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
| `make desktop` | 編譯 Rust 桌面 dashboard（release） |
| `make desktop-run` | 執行桌面 dashboard（debug 版） |
| `make desktop-test` | 執行桌面 dashboard 的測試 |
| `make desktop-lint` | 對 dashboard 執行 `cargo fmt --check` + `clippy` |

**範例**

```sh
make run ARGS="repo sync MilesChou/devpulse"
```
