# DevPulse

CI 與 PR 工作流程的研發效能觀測工具：從 GitHub 與 CI 服務抓資料、
聚合成團隊指標（CI 失敗率、PR review latency、build duration、
PR 重跑次數，以及四項 DORA 指標），寫入關聯式資料庫供後續分析使用。

以單一 Go binary 方式發佈。`devpulse serve` 會以 JSON API 提供指標和 repo
管理功能，另有一個選用的 Rust 桌面 dashboard（[`desktop/`](desktop/README.zh-TW.md)）
把指標畫成圖表。

> 英文版：[README.md](README.md)

## 定位

- **是什麼**：CLI 工具 + 關聯式資料層，上層再加一個 HTTP API 和桌面 dashboard
- **不是什麼**：SaaS、多租戶、即時 webhook 服務
- **適用規模**：單機、單一使用者、單月單 repo 約 100~1000 筆 build

## 安裝

### 環境需求

- Go **1.26+**（從原始碼編譯時才需要）
- 任一支援的資料庫：PostgreSQL、MySQL、SQLite（含 in-memory）
- GitHub personal access token；若有用 Travis 則需 Travis CI token
- Rust **1.95+**（只有編譯桌面 dashboard 時才需要）

### 從原始碼編譯

```bash
git clone https://github.com/MilesChou/devpulse.git
cd devpulse
make build
./bin/devpulse --help
```

或直接安裝：

```bash
go install github.com/mileschou/devpulse/cmd/devpulse@latest
```

### 編譯桌面 dashboard（選用）

Dashboard 是 `desktop/` 底下獨立的 Rust crate，不需要 Go toolchain。在要使用 dashboard 的機器上：

1. 用 [rustup](https://rustup.rs) 安裝 Rust 1.95 以上（已經裝過的話執行 `rustup update stable`）。
2. 安裝各平台的前置套件：macOS 裝 Xcode Command Line Tools，Windows 裝 Visual Studio Build Tools（C++ 工作負載），Debian / Ubuntu 則安裝：

   ```bash
   sudo apt-get install build-essential pkg-config \
     libxcb-render0-dev libxcb-shape0-dev libxcb-xfixes0-dev libxkbcommon-dev
   ```

3. 編譯：

   ```bash
   cd desktop
   cargo build --release --locked   # 或在 repo 根目錄執行 `make desktop`
   ```

4. 執行 `desktop/target/release/devpulse-desktop`（Windows 為 `devpulse-desktop.exe`），開啟 **Settings**，輸入正在運作的 `devpulse serve` 的 URL，server 有要求時再輸入 token。

細節請見 [desktop/README.zh-TW.md](desktop/README.zh-TW.md#編譯)：連到另一台機器上的 server、在本機完整試跑，以及疑難排解。

## 設定

複製範例檔並填入機密資訊：

```bash
cp .env.example .env
```

`DEVPULSE_DSN` 支援以下格式：

```
postgres://user:pass@host:5432/db?sslmode=disable
mysql://user:pass@host:3306/db?parseTime=true
sqlite://./devpulse.db?_fk=true
memory                              # in-memory SQLite，啟動時自動跑 migration
```

`memory` 模式不需任何外部服務，適合測試與一次性 CLI 跑。

## Quick Start

```bash
# 跑 migration（DEVPULSE_DSN=memory 會自動跑，這步可省略）
devpulse migrate up

# 註冊 repo
devpulse repo add MilesChou/devpulse

# 同步單一 repo：先撈所有 PR（含 review 與 enrichment），再從每個 CI provider
# 撈 build（GitHub Actions 必有；設定 TRAVIS_TOKEN 時加上 Travis CI）。
# 首次執行會打完整個歷史，會吃掉相當比例的 REST / GraphQL 配額；
# 後續執行為增量（per-provider watermark、upsert 去重、author backfill
# 略過已有的列）。
devpulse repo sync MilesChou/devpulse

# 或者一次跑完所有已註冊的 repo（循序執行；跳過 disabled 的 repo；
# 單一 repo 失敗不會中斷整批，最後彙整結果）。
devpulse sync

# 重新同步單一 PR（重抓 detail 與 reviews）
devpulse pr sync MilesChou/devpulse 42

# 顯示當月的工程效率指標
devpulse metrics MilesChou/devpulse --from 2026-05

# 啟動 worker 處理 enqueue 的 job（長時間執行）
devpulse worker

# 以 JSON 提供指標給桌面 dashboard（長時間執行）
DEVPULSE_API_TOKEN=change-me devpulse serve
```

選用的磁碟 HTTP 回應快取（`CACHE_ENABLED=true`）可以重播先前抓過的 API
回應——適合在不消耗 API 配額的情況下重建資料庫。快取相關變數與
`CACHE_TTL=0` replay 模式的注意事項，見
[docs/commands.zh-TW.md](docs/commands.zh-TW.md)。

## 用 Metabase 在本地探索資料

如要在本地以圖形介面瀏覽已同步的 build、PR、review 資料，可掛上選用的 Metabase overlay：

```bash
docker compose \
    -f docker-compose.yml \
    -f docker-compose.postgres.yml \
    -f docker-compose.metabase.yml up -d --wait
```

`metabase-init` sidecar 會自動完成這些設定：

- Admin 帳號：`admin@devpulse.local` / `changeme1!`（僅限本地 dev）
- 資料來源：DevPulse PostgreSQL，預設以 **DevPulse** 名稱掛好

`up -d --wait` 跑完即可——不需 first-run wizard、不需手填資料來源。接著開啟
[http://localhost:3000](http://localhost:3000) 登入。

若 init container 失敗結束（非零 exit code），用以下指令查看原因：

```bash
docker compose -f docker-compose.metabase.yml logs metabase-init
```

要重置 Metabase 從乾淨狀態開始：`docker compose down`，然後
`docker volume rm devpulse_metabase-data`，再 `up -d --wait`。Postgres 內的
資料（已同步的 PR、build、review）放在另一個 volume，不受影響。

## DORA 指標

`devpulse metrics` 會一併輸出四項 DORA 指標，資料都來自一般同步流程
已經抓下來的內容：

| 指標 | DevPulse 的計算方式 |
|---|---|
| Deployment Frequency（部署頻率） | merge 進 default branch 的 PR |
| Lead Time for Changes（介面上稱「commit 到部署」） | PR 中最早的 commit author 時間 → merge |
| Change Failure Rate（變更失敗率） | （revert + hotfix PR）÷ 部署數 |
| Recovery Time（恢復時間） | 被 revert 的 PR merge → revert merge，以及事故 issue 開啟 → 關閉 |

Hotfix PR 與事故 issue 以 label 辨識，可用
`devpulse repo config set <repo> hotfix-label <label>` 與
`incident-label <label>` 針對每個 repo 設定（預設為 `hotfix`、`incident`）。
完整定義與限制見 [docs/commands.zh-TW.md](docs/commands.zh-TW.md#dora-定義)。

## 桌面 dashboard

[`desktop/`](desktop/README.zh-TW.md) 是以 Rust（egui）寫成的原生 dashboard，
讀取 `devpulse serve` 的 API：CI 失敗率、每個 PR 的 build 次數、PR lead time
和 review 等待時間的 KPI 卡片（各附理想值和逐月變化）、四項 DORA 指標，
以及 PR 尺寸分布、建置時間中位數、12 個月的趨勢和依成員分項，每個畫面都可以
限定為某個團隊或成員。**Repos** 頁面可以新增、設定、同步和移除追蹤中的 repo；
**People** 頁面可以把 GitHub 帳號對應到成員和團隊，並設定哪些 bot 帳號不計入指標。

![桌面 dashboard](docs/images/desktop-dashboard.jpg)

```bash
DEVPULSE_API_TOKEN=change-me devpulse serve   # 在有資料庫的主機上執行
make desktop-run                              # 再到 Settings 輸入 URL（server 有要求時加上 token）
```

Dashboard 只保存 DevPulse API token（存在作業系統的 keychain）；GitHub 和
CI 的 token 只留在 server 上。端點和 JSON 格式請參考
[`serve`](docs/commands.zh-TW.md#serve)。

## 指令一覽

指令採 noun-on-verb 結構（`repo` / `pr` 兩個 resource group，動詞掛在
底下），風格與 `gh`、`jira-cli` 一致。`sync` 是唯一的頂層動詞，會對所有
已註冊的 repo fan-out，是 cron / CI 的天然入口。

| 指令 | 用途 |
|---|---|
| `devpulse sync` | 同步所有已註冊 repo（循序；跳過 disabled；彙整失敗） |
| `devpulse repo add <owner/name>` | 註冊一個 repo |
| `devpulse repo remove <owner/name> --yes` | 停止追蹤 repo 並刪除已同步的資料 |
| `devpulse repo sync <owner/name>` | 同步單一 repo：重抓 open 及上游有變動的 PR、新 PR（含 enrichment）、補齊既有 PR 缺少的 DORA 資料、CI build（並對應回所屬 PR）、事故 issue |
| `devpulse pr sync <owner/name> <number>` | 重新同步單一 PR（detail + reviews） |
| `devpulse metrics <owner/name>` | 印出月份區間的工程效率指標與 DORA 指標 |
| `devpulse migrate {up,down,status}` | Schema migration |
| `devpulse worker` | 啟動 DB-backed job worker |
| `devpulse serve` | 以 JSON API 提供指標和 repo 管理 |

## 開發

```bash
make all       # gofmt + go vet + go test + build（pre-commit hook 跑這個）
make build     # 編譯 binary 到 ./bin/devpulse
make test      # 跑 unit tests
make test-race # 跑 unit tests 並啟用 race detector
make lint      # gofmt + go vet
make tidy      # go mod tidy

make desktop-test  # 跑桌面 dashboard 的 cargo test
make desktop-lint  # cargo fmt --check + clippy
make desktop       # 編譯 release 版 dashboard
```

`make test` 預設打 in-memory SQLite。要對真實 PostgreSQL 或 MySQL 跑同一份測試，
把 `DEVPULSE_DSN` 指過去並串行執行（測試之間會 reset migrations，所以
並行跑會 race）：

```bash
DEVPULSE_DSN='postgres://devpulse:devpulse@localhost:5432/devpulse?sslmode=disable' \
  go test -p 1 -race -count=1 ./...
```

本機備有一組 Docker Compose overlay 可起本地後端。base 檔故意留空，
請挑一個（或同時開兩個）overlay：

```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d
docker compose -f docker-compose.yml -f docker-compose.mysql.yml    up -d
```

CI 會自動跑 SQLite、PostgreSQL、MySQL 三套 matrix — 詳見
[`.github/workflows/ci.yml`](.github/workflows/ci.yml)。

### Tracing

OpenTelemetry tracing 是可選的。把 `OTEL_EXPORTER_OTLP_ENDPOINT` 設成
collector 位址（例如本機 Jaeger 的 `localhost:4318`）就會把 span 送出去；
留空則 provider 是 no-op。

## 技術棧

- Go 1.26
- `database/sql` 搭配三個 driver：`jackc/pgx/v5/stdlib`、
  `go-sql-driver/mysql`、`modernc.org/sqlite`
- [`spf13/cobra`](https://github.com/spf13/cobra) 處理 CLI
- [`cli/go-gh`](https://github.com/cli/go-gh) 提供 GitHub HTTP client
  （帶預設 header 與 ASCII sanitizer；目前不會自動 retry）
- [`hashicorp/go-retryablehttp`](https://github.com/hashicorp/go-retryablehttp)
  處理 Travis HTTP client（以及其他通用對外 HTTP）
- OpenTelemetry SDK 提供 tracing
- in-tree 的 DB-backed job queue
- `net/http`（Go 1.22 以上的路由 pattern）提供 JSON API
- 桌面 dashboard：Rust，搭配 [`eframe`/`egui`](https://github.com/emilk/egui)、
  [`egui_plot`](https://github.com/emilk/egui_plot)、
  [`ureq`](https://github.com/algesten/ureq) 與
  [`keyring`](https://github.com/open-source-cooperative/keyring-rs)

## License

MIT — 詳見 [LICENSE](LICENSE)。
