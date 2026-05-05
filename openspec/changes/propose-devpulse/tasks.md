## 1. 專案基礎建置

- [x] 1.1 在本 repo 建立 Laravel 13 骨架（含 `composer.json`、`.env.example`）
- [x] 1.2 設定本機開發資料庫（先用 SQLite，可切 PostgreSQL）
- [x] 1.3 建立 `.env.example` 包含 `GITHUB_TOKEN`、`TRAVIS_TOKEN` placeholder 與註解說明取得方式
- [x] 1.4 建立 `config/devpulse.php` 包含 `excluded_bots`、`pr_size_buckets` 預設值
- [x] 1.5 設定 PHPUnit、PHPCS、PHPStan、GitHub Actions CI

## 2. 設定機制

- [x] 2.1 建立 `members`、`groups`、`repos`、`group_repos`、`group_members` 等 migration
- [x] 2.2 建立對應 Eloquent Model
- [x] 2.3 建立 Artisan command `devpulse:group:create`、`devpulse:member:add`、`devpulse:repo:add`

## 3. CI 資料抓取

- [x] 3.1 定義 CI provider 抽象介面（含 `listBuildsInMonth`、`getBuildLog` 等核心操作）
- [x] 3.2 定義 `BuildSummary` value object（`final readonly class`，constructor 驗證不變式，封裝 `isPostMerge()`、`isPullRequest()`、`isDeployEvent()`、`isFailure()` 等業務判斷；放於 `app/Domain/Ci/`）
- [x] 3.3 在 `AppServiceProvider` 把 CI provider 抽象介面預設綁定到 Travis 實作
- [x] 3.4 實作 Travis provider，包含 API client（Saloon 或 Guzzle）、token 注入
- [x] 3.5 在 Travis provider 實作 `BuildSummary::fromTravisRaw()` named constructor，把原生欄位翻譯成 VO（如 event_type=push + branch=master → `isPostMerge()` 為真）
- [x] 3.6 加上 retry middleware 處理 rate limit / 5xx
- [x] 3.7 撰寫單元測試：VO 不變式違反時 throw、`isPostMerge()` 等規則正確
- [x] 3.8 撰寫整合測試：用 cassette 或 mock server 模擬 Travis 回應

## 4. GitHub 資料抓取

- [x] 4.1 定義 `PullRequestSummary`、`ReviewSummary` value object（`final readonly class`，含 ready_at、first_review_at、author、status、行數等欄位；放於 `app/Domain/Vcs/`）
- [x] 4.2 實作 GitHub client（PR 查詢、PR 詳細、commit author bulk、PR head ref bulk）
- [x] 4.3 實作 PR review 資料抓取（含 ready_at、first_review_at），需用 GraphQL
- [x] 4.4 在 GitHub client 實作 `PullRequestSummary::fromGitHubRaw()` 等 named constructor，把原生欄位翻譯成 VO
- [x] 4.5 加上 retry / rate limit 處理
- [x] 4.6 加上 bot author / reviewer 過濾（依 `excluded_bots` 設定）
- [x] 4.7 撰寫單元測試：VO 不變式違反時 throw、bot 過濾邏輯正確
- [x] 4.8 撰寫整合測試：mock GitHub 回應驗證解析正確

## 5. 持久化資料層

- [x] 5.1 建立 `builds` table migration（含 raw_payload JSON 欄位）
- [x] 5.2 建立 `pull_requests` table migration（含 raw_payload）
- [x] 5.3 建立 `month_fetches` table 記錄每個 (repo, month) 的撈取狀態（complete / partial）
- [x] 5.4 實作 VO ↔ Eloquent Model 的 hydrator / mapper（VO 不繼承 Model，兩者用 mapper 轉換）
- [x] 5.5 實作 fetcher 寫入時的 upsert 邏輯（同 build_id 不重複）
- [x] 5.6 實作「已過月份不重撈」的 cache decision 邏輯
- [x] 5.7 撰寫測試：第二次跑同月份不打外部 API（MonthFetchCache 單元測試完成；orchestrator e2e 測試在第 6 章 aggregator 階段補）

## 6. 聚合層

- [x] 6.1 實作「單月成員 × repo 失敗率」聚合
- [x] 6.2 實作預設排除 post-merge / deploy event 的 filter
- [x] 6.3 實作 PR review latency 計算（含 month_cutoff lower bound）
- [x] 6.4 實作 PR size 分桶（依 config 設定）
- [x] 6.5 實作月對月對比（含 ↑↓→ 方向計算）
- [x] 6.6 實作 daily passed duration 聚合
- [x] 6.7 實作 PR 重跑次數聚合
- [x] 6.8 撰寫單元測試：每個 aggregator 都有對應測試 fixture
- [x] 6.9 撰寫整合測試：指定不同 group 跑 aggregator，能取得對應的 repos 與 members 切片（對應 tool-configuration spec「切換 group 看不同團隊」場景）

## 7. 失敗 build 分類

- [x] 7.1 在 `repos` table 加上 `human_signals`（JSON 欄位）
- [x] 7.2 實作 classifier：依 signal 規則對失敗 build 的 log 做字串比對
- [x] 7.3 撰寫測試：lint signal 命中時應分類為 human/lint

## 8. 報告產出

- [x] 8.1 實作 markdown renderer：失敗率月報表格
- [x] 8.2 實作 markdown renderer：PR review latency 表格
- [x] 8.3 實作 markdown renderer：daily build duration 趨勢
- [x] 8.4 實作 markdown renderer：失敗 build 清單
- [x] 8.5 實作 Artisan command `devpulse:report <month> [--group=] [--output=]`
- [x] 8.6 撰寫端到端測試：跑指令能產生預期 markdown

## 9. Golden output 對照

- [ ] 9.1 用 Python prototype（`ci_analysis`）跑出某月 markdown 報告作為 golden
- [ ] 9.2 用相同月份 + 設定跑 PHP 版，比對失敗率、PR review latency、build 計數的數值
- [ ] 9.3 文件化「兩版本必須一致」的核心數值清單
- [ ] 9.4 若有差異，紀錄差異原因（時區、邊界處理、bug）並修正 PHP 版

## 10. 文件與收尾

- [x] 10.1 撰寫 README：定位、安裝、初次設定、跑第一份月報的 quick start
- [x] 10.2 撰寫 `docs/group-setup.md`：如何設定 group / members / repos / human_signals
- [x] 10.3 撰寫 `docs/migration-from-prototype.md`：給未來的自己看，什麼時候 retire Python prototype
- [x] 10.4 加 LICENSE（建議 MIT）
- [x] 10.5 確認 `.gitignore` 排除 `.env`、本機 cache 目錄

