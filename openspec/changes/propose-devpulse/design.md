## Context

devpulse 是繼承自 Python prototype（`ci_analysis`）的「正規版本」研發效能觀測工具，目標是把 prototype 階段驗證過的 domain knowledge 落實成長期維護、可重複部署於不同團隊的工具。

**繼承自 prototype 的 domain knowledge：**
- 已驗證可從 Travis API 撈 build 歷史、從 GitHub API 撈 PR / review 資料
- 已釐清哪些 build 要排除（post-merge / deploy event）、哪些 bot PR 要排除、PR size 如何分桶、human signal 規則如何定義
- 月對月對比、daily passed duration、PR review latency 等指標的算法已驗證

**主要約束：**
- 主開發語言為 PHP，長期維護成本是首要考量
- side project 性質，沒有上線壓力，可慢慢演進
- 必須能於不同團隊 / 環境部署，不綁定任何特定組織的設定

## Goals / Non-Goals

**Goals：**
- 用 PHP Laravel 重寫 ci_analysis 的資料抓取與聚合層
- 建立 PostgreSQL 持久化資料層（取代 file cache），讓資料可長期累積、可 query
- 建立 CI provider 抽象，讓未來新增 GitHub Actions 等 provider 不需改 caller
- 第一階段產出與 Python prototype 等價的 markdown 月報（用於 golden output 對照）
- 設定機制與特定組織脫鉤（範例設定僅含中性 placeholder）

**Non-Goals：**
- Web Dashboard（Stage 2 才考慮）
- CLI binary 跨平台分發（Stage 3 才考慮，且可能改用 Go）
- 認證 / 權限模型（單機自用階段不需要）
- 即時 webhook 接收（純歷史資料分析，不做即時）
- 多租戶 / SaaS 化（明確排除）
- 完全取代 Python prototype（保留為 reference / golden output）

## Decisions

### Decision 1：使用 Laravel（PHP）而非繼續 Python / 改用 Go / TS

**選擇：** Laravel 11/12（PHP 8.x）

**理由：**
- PHP 是主開發語言，長期維護成本最低
- Laravel 內建 queue、scheduler、cache、HTTP client（Guzzle）、Eloquent ORM、Artisan command，完整覆蓋本工具所需能力
- side project 最大風險是棄坑，用熟悉的工具能讓迭代速度跟得上動力

**替代方案：**
- 繼續 Python：勉強可行，但長期維護心智成本高
- Go：CLI 取向最佳，但 Web 階段又得換語言、生態學習成本大
- TypeScript：全棧統一，但 SaaS 級工程量需要相當熟練度
- 維持 Python prototype 不重寫：prototype 與其他筆記 / 雜項並存，不利長期維護與重複部署

### Decision 2：先做資料抓取層而非 Web UI

**選擇：** Stage 1 只做 fetcher + aggregator + markdown 輸出，不碰 Web

**理由：**
- 「先資料後展示」避開兩套 data layer 並行的同步地獄
- markdown 輸出可以跟 Python prototype 對照，快速驗證資料正確性
- Web UI 是大投資，要等資料層穩定再上

**替代方案：**
- 先做 Web shell，data 從 Python prototype 餵進來：兩套 data layer 同步是經典反模式
- 一次做完 CLI + Web：超出 side project 可承受工作量

### Decision 3：用 PostgreSQL 取代 file-based cache

**選擇：** PostgreSQL 為持久化儲存，使用 Eloquent

**理由：**
- Python prototype 的 file cache 只支援「by key」存取，無法 query / 聚合 / 跨月趨勢
- 將來 Web 需要 query 趨勢時 DB 是必經之路，提早設計
- 開發階段可以 SQLite，production 階段切 PostgreSQL（Laravel migration 兼容）

**替代方案：**
- 維持 file cache：簡單但限制大，將來必然要重做
- ClickHouse：時序資料效能更好，但本工具的資料量遠未達到需要
- Redis：適合 cache 不適合 source of truth

### Decision 4：CI provider 抽象介面從第一版就建立

**選擇：** 定義 `CIProviderInterface`，第一版只實作 `TravisProvider`

**理由：**
- Python prototype 已驗證抽象的形狀（`providers/base.py` + `providers/travis.py`）
- 業界趨勢上 GitHub Actions 是必然要支援的下一個 provider
- 從第一版就抽象比之後重構便宜
- Laravel Service Container 天生適合 bind interface

**替代方案：**
- 先寫死 Travis、未來再抽象：但 Python prototype 證明這個抽象成本不高、收益清楚
- 一次支援多 provider：Stage 1 太貪心

### Decision 5：設定機制：DB + config 雙層

**選擇：**
- 不變的、跨環境共用的（PR size buckets、bot 排除清單）→ `config/devpulse.php`
- 會變的、跟組織/團隊綁定的（people、repos、profiles）→ DB table（可由 seeder / artisan command 管理）

**理由：**
- Python prototype 把所有東西堆在 `.profile.yaml` 裡，部署到不同團隊時要手動修改 yaml，繁瑣
- DB 化的好處：將來 Web UI 可以直接編輯、可以多 profile 並存

**替代方案：**
- 全部塞 yaml：與 prototype 一樣的痛
- 全部塞 config：靜態化，多 profile / 動態切換麻煩

### Decision 6：Python prototype 保留作為 reference / golden output

**選擇：** ci_analysis 維持原狀運作；devpulse 第一版的驗證標準是「同月份、同設定下，產出與 Python 一致的 markdown 報告」

**理由：**
- Joel Spolsky 的「不要重寫」鐵則的緩衝：保留可運作 reference 在重寫期間就有對照
- 重寫過程中發現 Python 版邏輯有 bug 也能即時對照
- Python 版的 cache 可以當作測試 fixture（不用每次都打外部 API）

**替代方案：**
- 直接刪 Python 版：失去 golden output 對照，重寫風險大幅升高
- 寫成單向遷移工具：超出 Stage 1 範圍

## Risks / Trade-offs

- **Risk: Travis API 撈大量歷史資料慢、容易 rate limit** → Mitigation: 用 Laravel Queue + 重試、cache HTTP response（檔案層級，跟 Python prototype 同樣策略）、撈過的月份標記為 immutable 不重撈
- **Risk: 月報產出與 Python prototype 對不起來** → Mitigation: 第一版每個 aggregator 函式都針對同月份資料寫對照測試，PHP 算的 stats 必須等於 Python 算的 stats
- **Risk: PHP/Carbon 對 ISO 8601 'Z' 後綴的解析行為跟 Python `fromisoformat` 不同** → Mitigation: 在 fetcher 邊界統一把所有時間解析為 UTC Carbon instance，aggregator 不直接碰字串
- **Risk: schema 第一版設計錯了之後改很痛** → Mitigation: 先參考 Python prototype 的 dict 結構反推 schema、保留所有 raw response（一個 JSONB 欄位）以便將來補欄位、用 migration 演進
- **Risk: side project 動力消退而棄坑** → Mitigation: Stage 1 範圍極度收斂（不做 Web、不做 CLI 分發、不做 auth），確保 1~2 個月內能跑出有用的東西
- **Risk: Python prototype 與 PHP 版本長期並存導致心智雙重負擔** → Mitigation: 明確只把 Python 當 reference，Stage 1 結束後若 PHP 版本能跑出等價結果，就停止 Python 版本的功能演進

## Migration Plan

本 repo 為空，無既有資料 / 使用者，無遷移問題。

僅需確認：
- Python prototype（`ci_analysis`）保留可執行狀態，作為 reference
- devpulse 第一版的 settings 結構不要與 Python 的 `.profile.yaml` 強相依（不要做 import yaml 工具，會綁住 schema）

## Open Questions

- HTTP client 選 Guzzle 直用還是 Saloon（Laravel 友善的 API client wrapper）？傾向 Saloon，待 Stage 1 開始時確認
- aggregator 的 markdown 模板要不要從 Python prototype 的 `report/writer.py` 直接抄？傾向不抄、重新設計，因為 Python 版的 Obsidian transclusion 機制不一定適用
- 是否提供 Python prototype 的 `.cache/` 目錄轉換工具，讓 PHP 版本能直接吃既有 cache？暫定不做（增加 scope）

