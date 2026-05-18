# DevPulse

研發效能觀測工具：從 GitHub 與 CI 抓資料、聚合成團隊指標
（CI 失敗率、PR review latency、build duration、PR 重跑次數），
寫入關聯式資料庫供後續分析使用。

從 Python prototype 與 Laravel 版本演進而來，目前以 Go 重寫為單一 binary。

## 定位

- **是什麼**：CLI 工具 + 關聯式資料層
- **不是什麼**：SaaS、多租戶、即時 webhook 服務
- **量級**：單機、單一使用者、單月單 repo 約 100~1000 筆 build 的小資料量

## 安裝

### 環境需求

- Go **1.26+**（編譯時）
- 任一支援的資料庫：PostgreSQL、MySQL、SQLite（含 in-memory）
- GitHub personal access token、Travis CI token

### 從原始碼編譯

```bash
git clone https://github.com/MilesChou/devpulse.git
cd devpulse
make build
./bin/devpulse --help
```

或直接用 `go install`：

```bash
go install github.com/mileschou/devpulse/cmd/devpulse@latest
```

## 設定

複製 `.env.example` 並填入：

```bash
cp .env.example .env
```

`DEVPULSE_DSN` 支援以下格式：

```
postgres://user:pass@host:5432/db?sslmode=disable
mysql://user:pass@host:3306/db?parseTime=true
sqlite://./devpulse.db?_fk=true
memory                              # in-memory SQLite，每次重啟自動跑 migration
```

`memory` 模式不需任何外部服務，適合測試與一次性 CLI 跑。

## Quick Start

```bash
# 跑 migration（in-memory 會自動跑，這步可省略）
devpulse migrate up

# 註冊 repo
devpulse repo-add MilesChou/devpulse

# 撈某月份的 build / PR / review 與計算 lead-time
devpulse fetch MilesChou/devpulse 2026-05

# 針對單一 PR 重新算 enrichment
devpulse enrich-pr MilesChou/devpulse 42

# 啟動 worker 處理 enqueue 的 job
devpulse worker
```

## 指令一覽

| 指令 | 用途 |
|---|---|
| `devpulse migrate {up,down,status}` | Schema migration |
| `devpulse repo-add <owner/name>` | 註冊一個 repo |
| `devpulse fetch <owner/name> <YYYY-MM>` | 抓某月的 build + PR + review |
| `devpulse enrich-pr <owner/name> <number>` | 重新計算單一 PR 的 enrichment |
| `devpulse worker` | 啟動 DB-backed job worker |
| `devpulse serve` | (placeholder，v2 提供 HTTP API) |

## 開發

```bash
make build          # 編譯 binary 到 ./bin/devpulse
make test           # 跑 unit tests
make test-race      # 跑 unit tests 含 race detector
make test-integration  # 跑 integration tests（需 Docker）
make lint           # gofmt + go vet
make tidy           # go mod tidy
```

OpenTelemetry tracing 可選：把 `OTEL_EXPORTER_OTLP_ENDPOINT` 設成本機 Jaeger
（`localhost:4318`）就能在 dev 環境看 span。

## 技術棧

- Go 1.26
- `database/sql` + 多 driver（pgx、go-sql-driver/mysql、modernc.org/sqlite）
- spf13/cobra（CLI）
- hashicorp/go-retryablehttp（HTTP retry）
- OpenTelemetry SDK（tracing）
- 自寫 DB-backed job queue

## License

MIT — 詳見 [LICENSE](LICENSE)

