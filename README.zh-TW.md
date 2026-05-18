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

# 撈某月份的 CI build
devpulse build fetch MilesChou/devpulse 2026-05

# 撈同一個月的 PR + review + enrichment
devpulse pr fetch MilesChou/devpulse 2026-05

# 針對單一 PR 重新計算 enrichment
devpulse pr enrich MilesChou/devpulse 42

# 啟動 worker 處理 enqueue 的 job（長時間執行）
devpulse worker
```

## 指令一覽

指令採 noun-on-verb 結構（`repo` / `build` / `pr` 三個 resource group，
動詞掛在底下），風格與 `gh`、`jira-cli` 一致。

| 指令 | 用途 |
|---|---|
| `devpulse repo add <owner/name>` | 註冊一個 repo |
| `devpulse build fetch <owner/name> <YYYY-MM>` | 撈某月份的 CI build |
| `devpulse pr fetch <owner/name> <YYYY-MM>` | 撈某月份的 PR + review + enrichment |
| `devpulse pr enrich <owner/name> <number>` | 重新計算單一 PR 的 enrichment |
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
- [`hashicorp/go-retryablehttp`](https://github.com/hashicorp/go-retryablehttp)
  處理對外 HTTP
- OpenTelemetry SDK 提供 tracing
- in-tree 的 DB-backed job queue

## License

MIT — 詳見 [LICENSE](LICENSE)。
