## Why

已存在一個 Python prototype（`ci_analysis`），能撈 Travis CI build 與 GitHub PR review 歷史資料、產出月對月對比的研發效能報告，並已驗證 domain know-how。但該 prototype 屬於 side project，與其他筆記 / 雜項並存於同一個 repo、綁定特定組織的設定，無法成為長期維護、可重複部署於不同團隊的「正規工具」。需要一個全新起點（devpulse）以 PHP Laravel 重寫資料抓取與聚合層，把驗證過的 domain knowledge 沉澱成可長期演進、最終支援 CLI + Web 的研發效能觀測平台。

## What Changes

- 新增 `devpulse` 為本 repo 的工具名稱（取代原本探索性的 `coverdiff`），定位為「研發效能觀測工具」
- 新增 PHP Laravel 為主技術棧（取代 Python prototype 的角色），使用最新 LTS 版本
- 新增資料抓取層：GitHub API（PR、commit author、PR review）+ Travis API（build 歷史）
- 新增 CI provider 抽象介面，第一版只實作 Travis，預留 GitHub Actions 等未來擴充
- 新增持久化資料庫（PostgreSQL），取代 Python prototype 的 file-based cache
- 新增聚合與報告產出機制（先以 Artisan command 產出 markdown，不做 Web UI）
- 新增 `.profile.yaml` 等價的設定機制（config + database），但拔掉特定組織的預設值
- Python prototype（ci_analysis）保留為 reference / golden output 對照來源，不直接依賴或匯入
- 非目標（暫不做）：Web Dashboard、CLI binary 分發、認證/權限模型、即時 webhook 接收

## Capabilities

### New Capabilities

- `vcs-data-fetching`: 從 GitHub API 撈 PR、commit author、PR review 的歷史資料，含 rate limit 處理、bulk fetching、bot 過濾
- `ci-data-fetching`: 從 CI provider（第一版 Travis）撈 build 歷史資料，含 build 屬性翻譯（is_post_merge / is_pull_request / is_deploy_event）
- `ci-provider-abstraction`: 定義 CIProviderInterface，讓未來新增 GitHub Actions / CircleCI 等 provider 不需改動 caller
- `metrics-aggregation`: 把 raw build / PR / review 資料聚合成研發效能指標（CI 失敗率、PR review latency、PR size 分桶、月對月對比）
- `metrics-persistence`: 把抓回的 raw data 與聚合結果持久化到 PostgreSQL，支援後續查詢與長期累積
- `report-rendering`: 將聚合結果以 markdown 形式輸出（為將來 Web UI 預留聚合層 / view layer 分離）
- `tool-configuration`: 提供 `.profile.yaml` 等價的設定機制，定義成員清單、repo 清單、bot 排除名單、PR size 分桶、human signal 規則

### Modified Capabilities

（無——本 repo 為空，所有 capability 皆為新增）

## Impact

- **程式碼**：本 repo 從零建立，採 Laravel 標準骨架
- **依賴**：PHP 8.x、Laravel 11/12、PostgreSQL、Guzzle/Saloon（HTTP client）、Carbon（datetime）
- **外部 API**：GitHub REST/GraphQL API、Travis CI API（需 personal token）
- **設定**：新增 `.env`（API token）、`config/devpulse.php`（profile 設定）
- **參考來源**：Python prototype（`ci_analysis`），保留不動，僅作為對照
- **不影響**：Python prototype 所在 repo 的既有運作；prototype 仍可獨立執行

