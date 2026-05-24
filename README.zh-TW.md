# DevPulse

CI 與 PR 工作流程的研發效能觀測工具：從 GitHub 與 CI 服務抓資料、
聚合成團隊指標（CI 失敗率、PR review latency、build duration、
PR 重跑次數），寫入關聯式資料庫供後續分析使用。

以單一 Go binary 方式發佈。

> 英文版：[README.md](README.md)

## 定位

- **是什麼**：CLI 工具 + 關聯式資料層
- **不是什麼**：SaaS、多租戶、即時 webhook 服務
- **適用規模**：單機、單一使用者、單月單 repo 約 100~1000 筆 build

## 安裝

### 環境需求

- Go **1.26+**（從原始碼編譯時才需要）
- 任一支援的資料庫：PostgreSQL、MySQL、SQLite（含 in-memory）
- GitHub personal access token；若有用 Travis 則需 Travis CI token

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

# 同步單一 repo：先撈所有 PR（含 review 與 enrichment），再撈所有 CI build。
# 首次執行會打完整個 GitHub 與 Travis 歷史，會吃掉相當比例的 REST / GraphQL
# 配額；後續執行為增量（upsert 去重、author backfill 略過已有的列）。
devpulse repo sync MilesChou/devpulse

# 或者一次跑完所有已註冊的 repo（循序執行；跳過 disabled 的 repo；
# 單一 repo 失敗不會中斷整批，最後彙整結果）。
devpulse sync

# 重新同步單一 PR（重抓 detail 與 reviews）
devpulse pr sync MilesChou/devpulse 42

# 啟動 worker 處理 enqueue 的 job（長時間執行）
devpulse worker
```

## 用 Metabase 在本地探索指標

如要在本地以圖形介面瀏覽 metric view（見
`migrations/*views_failure_rate.up.sql`），可掛上選用的 Metabase overlay：

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

## 指令一覽

指令採 noun-on-verb 結構（`repo` / `pr` 兩個 resource group，動詞掛在
底下），風格與 `gh`、`jira-cli` 一致。`sync` 是唯一的頂層動詞，會對所有
已註冊的 repo fan-out，是 cron / CI 的天然入口。

| 指令 | 用途 |
|---|---|
| `devpulse sync` | 同步所有已註冊 repo（循序；跳過 disabled；彙整失敗） |
| `devpulse repo add <owner/name>` | 註冊一個 repo |
| `devpulse repo sync <owner/name>` | 同步單一 repo：所有 PR（含 enrichment）與 CI build |
| `devpulse pr sync <owner/name> <number>` | 重新同步單一 PR（detail + reviews） |
| `devpulse migrate {up,down,status}` | Schema migration |
| `devpulse worker` | 啟動 DB-backed job worker |
| `devpulse serve` | v2 HTTP API 的 placeholder |

## 開發

```bash
make all       # gofmt + go vet + go test + build（pre-commit hook 跑這個）
make build     # 編譯 binary 到 ./bin/devpulse
make test      # 跑 unit tests
make test-race # 跑 unit tests 並啟用 race detector
make lint      # gofmt + go vet
make tidy      # go mod tidy
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

## License

MIT — 詳見 [LICENSE](LICENSE)。
