# Grafana for devpulse

本機 Grafana，連 devpulse 的 PostgreSQL，提供 CI 失敗率、PR review latency、build duration、PR 數量/size 四個面板。

## 啟動

```bash
cd grafana
cp .env.example .env
# 編輯 .env 填入你的 PG DSN（host / port / db / user / password）
docker compose up -d
```

開 <http://localhost:3000>，預設帳密 `admin / admin`（可在 `.env` 改）。
左側 Dashboards → devpulse → **devpulse overview**。

## 連線提醒（macOS / Windows）

PG 跑在你 host 機器（localhost）的話，`DEVPULSE_DB_HOST` 要填 `host.docker.internal`，**不是** `localhost`。
Linux 沒有 `host.docker.internal`，請填 host 在 docker bridge 上的 IP（通常 `172.17.0.1`），或把 PG 也放進 docker compose。
遠端 PG 直接填實際 host。

## 結構

```
grafana/
  docker-compose.yml                       # Grafana service
  .env.example                             # PG DSN 範本（.env 由 .gitignore 排除）
  provisioning/
    datasources/devpulse.yml               # PG datasource，從環境變數注入
    dashboards/devpulse.yml                # dashboard provider 設定
  dashboards/
    devpulse-overview.json                 # 四個面板的 dashboard 本體
```

## 面板對應的 SQL 來源

| 面板 | 對應 Aggregation Query |
| --- | --- |
| CI 失敗率（每 repo / 月） | `BuildFailureRateQuery` |
| PR review latency（依 size bucket） | `ReviewLatencyQuery` |
| 每日 Build duration（p50 / p95） | `DailyBuildDurationQuery` |
| PR 數量與 size 分佈 | `PrBuildCountQuery` + `PrSizeBucket` |

SQL 直接寫在 dashboard JSON 裡，方便你打開 Grafana UI 改 query 微調。

## 修改 dashboard

`updateIntervalSeconds: 30` + `allowUiUpdates: true` 已開啟，你可以直接在 UI 改面板，再用「Share → Export → Save to file」把 JSON 覆蓋回 `dashboards/devpulse-overview.json`。

