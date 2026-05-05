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

**選擇：** Laravel 13（PHP 8.3+，骨架建立當下安裝為 13.x）

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

### Decision 4：在 ci-data-fetching capability 內部以 provider 抽象隔離具體 CI 服務

**選擇：** 在 `ci-data-fetching` capability 內部定義抽象介面，第一版只實作 Travis 一個 provider；不把 provider 抽象本身單獨列為一個 capability

**理由：**
- Python prototype 已驗證抽象的形狀（`providers/base.py` + `providers/travis.py`）
- 業界趨勢上 GitHub Actions 是必然要支援的下一個 provider
- 從第一版就抽象比之後重構便宜
- Laravel Service Container 天生適合 bind interface
- 抽象本身是 ci-data-fetching 的內部結構決策，不需要在 spec 層暴露為獨立 capability，避免概念過細

**替代方案：**
- 先寫死 Travis、未來再抽象：但 Python prototype 證明這個抽象成本不高、收益清楚
- 一次支援多 provider：Stage 1 太貪心
- 把 provider 抽象拆為獨立 capability：曾考慮過，但這拉高了 spec 的層級切分粒度，使用者視角看到的是「能不能撈 CI 資料」，介面是不是抽象屬於實作層

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

### Decision 7：view layer 採 markdown + Grafana dashboard 雙軌

**選擇：** 第一版輸出維持 markdown report，將來新增 Grafana dashboard 作為第二個 view layer 直連 PostgreSQL；不自寫 web dashboard、不把 markdown 作為終極唯一輸出

**重新定位：** markdown 的角色從「終極輸出」收斂為「reference 對照 + archive 用途」；dashboard 才是日常閱讀的終極形式

**理由：**
- Stage 1 終極目標是「有 dashboard 可看」，markdown 是過渡形式不是目標
- Grafana 直連 PostgreSQL（Decision 3 已選），純 view layer，不重複任何 ETL / aggregator 工作
- 自寫 web dashboard 在 PHP 為主、前端非主修的脈絡下邊際成本太高，會成為 Stage 2 的大坑
- markdown 仍要做：兼任 golden output 對照工具（Decision 6）與 wiki / commit 用途
- Grafana dashboard 是 SQL view + dashboard JSON，可 version control、可重複部署於不同團隊
- 互動式 filter / variable 支援多 profile 切換，比 markdown 更貼近實際使用情境

**適配度評估：**
- 月對月對比、daily build duration 趨勢、失敗率 by 成員 × repo、PR review latency by size bucket、失敗 build 清單（含 PR 連結）→ Grafana panel 完全勝任
- 失敗 build human signal 分類、bot 過濾、CI 屬性翻譯、cache 邏輯 → 仍由 devpulse aggregator / fetcher 做完寫進 DB，Grafana 只讀

**替代方案：**
- 完全用 Grafana 取代 devpulse：不可行，Grafana 不抓資料、不做業務邏輯分類
- 完全不用 Grafana、自寫 web dashboard：Stage 2 工作量爆表，且前端不在熟練語言範圍內
- 只做 markdown 不做 dashboard：違背「目標是 dashboard」的初衷

**Trade-offs：**
- 多一個 Grafana 部署依賴（Docker compose 多一個服務）
- dashboard JSON 比 markdown 不直觀，需 version control 紀律
- 客製化呈現（獨特 metric 公式）會被 Grafana 視覺化能力限制

**範圍邊界：** Stage 1 不寫任何 Grafana dashboard，僅在 PostgreSQL schema 上預留方便 query 的 view 層思維（避免 schema 設計成只方便 Laravel ORM 用）。Grafana dashboard 設計與部署留待 Stage 2，新增獨立 capability `dashboard-rendering`

### Decision 8：Domain 物件採 ValueObject 而非貧血 DTO

**選擇：** 跨層流動的 domain 物件（如 build summary、PR summary、review summary 等），用 PHP 8.4 `final readonly class` 寫成 ValueObject：constructor 收 raw 值並驗證不變式（invariants）、封裝業務判斷行為（如 `isPostMerge()`、`isPullRequest()`、`isDeployEvent()`），不只是 getter

**理由：**
- 多入口場景：CI provider 抽象（Decision 4）讓未來會有 Travis、GitHub Actions 等多個 provider 都會建出 build summary VO，在 VO 層統一驗證比每個 provider 各驗一次更可靠
- 不變式違反代表上游 bug：例如 `commitSha` 為空、`durationSeconds` 為負，這些是 fetcher / provider translation 的錯，VO constructor throw 能在最早的時間點爆炸而非在 aggregator 算出怪結果
- 業務判斷規則集中：`isPostMerge()` 的定義（event_type=push + branch=master）只在一個地方，aggregator 不會散落多份規則
- 與 Risks 段「PHP/Carbon ISO 8601 解析行為跟 Python 不同」的緩解一致 —— VO 接受 `CarbonImmutable` 而非 raw string，型別系統強制呼叫者在邊界完成解析
- side project 沒上線壓力，多寫幾行 invariant validation 不會拖慢進度，但能在重寫過程中快速發現「Python 跟 PHP 看到的資料不一樣」

**設計原則：**
- VO 為 `final readonly class`，所有 property `public readonly`，無 setter
- constructor 拋 `InvalidArgumentException`（或 domain-specific exception）擋住不可能存在的狀態
- 建構工廠用 named constructor：例如 `BuildSummary::fromTravisRaw($payload)`，把 provider 的格式轉換責任放在 provider 而非 VO
- 業務判斷用 method（`isFailure()`），不外露 raw enum 比較
- 時間欄位用 `CarbonImmutable`、不用 string；錢 / 數量等用 typed value（Stage 1 暫不需要，但保留設計空間）

**替代方案：**
- 貧血 DTO（只有 `public readonly` 屬性、沒有行為）：節省幾行，但業務規則散落 aggregator、多入口時驗證重複；對 side project 維護不利
- 直接用 Eloquent Model 當 domain 物件：Eloquent 是 persistence 工具不是 domain 工具，attribute 預設 mutable、跟 DB schema 強耦合；持久化層（Decision 3）與 domain 層應分開
- 不寫 invariant validation、純靠 type system + 邊界驗證：選項可行但 multi-provider 場景下，邊界驗證重複；VO 自我保護更穩

**Trade-offs：**
- VO 與 Eloquent Model 並存：需要 hydrator / mapper 在兩者之間轉換（Stage 1 寫一次，後續 reuse）
- 多寫一層類別：對「只 Stage 1 跑跑看就停」的開發者顯得繁瑣；但本工具明確是「長期維護」目標（Decision 1 理由），值得這成本

**範圍邊界：** Stage 1 至少建立 `BuildSummary`、`PullRequestSummary`、`ReviewSummary` 三個 VO；其他物件視需要新增。VO 放於 `app/Domain/` 命名空間，與 Eloquent Model（`app/Models/`）分開

## Risks / Trade-offs

- **Risk: Travis API 撈大量歷史資料慢、容易 rate limit** → Mitigation: 用 Laravel Queue + 重試、cache HTTP response（檔案層級，跟 Python prototype 同樣策略）、撈過的月份標記為 immutable 不重撈
- **Risk: 月報產出與 Python prototype 對不起來** → Mitigation: 第一版每個 aggregator 函式都針對同月份資料寫對照測試，PHP 算的 stats 必須等於 Python 算的 stats
- **Risk: PHP/Carbon 對 ISO 8601 'Z' 後綴的解析行為跟 Python `fromisoformat` 不同** → Mitigation: 在 fetcher 邊界統一把所有時間解析為 UTC Carbon instance，aggregator 不直接碰字串
- **Risk: schema 第一版設計錯了之後改很痛** → Mitigation: 先參考 Python prototype 的 dict 結構反推 schema、保留所有 raw response（一個 JSONB 欄位）以便將來補欄位、用 migration 演進
- **Risk: side project 動力消退而棄坑** → Mitigation: Stage 1 範圍極度收斂（不做 Web、不做 CLI 分發、不做 auth），確保 1~2 個月內能跑出有用的東西
- **Risk: Python prototype 與 PHP 版本長期並存導致心智雙重負擔** → Mitigation: 明確只把 Python 當 reference，Stage 1 結束後若 PHP 版本能跑出等價結果，就停止 Python 版本的功能演進
- **Risk: Stage 1 為 markdown 設計的聚合層 schema，在 Grafana dashboard 上 query 起來不順** → Mitigation: schema 設計階段就把「直接 SQL query 也要好寫」當成設計原則之一，避免把所有東西都塞進 raw_payload JSON 欄位、需要的維度欄位拉出來成獨立欄位

## Migration Plan

本 repo 為空，無既有資料 / 使用者，無遷移問題。

僅需確認：
- Python prototype（`ci_analysis`）保留可執行狀態，作為 reference
- devpulse 第一版的 settings 結構不要與 Python 的 `.profile.yaml` 強相依（不要做 import yaml 工具，會綁住 schema）

## Open Questions

- HTTP client 選 Guzzle 直用還是 Saloon（Laravel 友善的 API client wrapper）？傾向 Saloon，待 Stage 1 開始時確認
- aggregator 的 markdown 模板要不要從 Python prototype 的 `report/writer.py` 直接抄？傾向不抄、重新設計，因為 Python 版的 Obsidian transclusion 機制不一定適用
- 是否提供 Python prototype 的 `.cache/` 目錄轉換工具，讓 PHP 版本能直接吃既有 cache？暫定不做（增加 scope）
- Grafana 部署形式：Docker compose 跟 PostgreSQL 一起起、還是另外裝一份共用？傾向 Docker compose，待 Stage 2 開始 dashboard-rendering 時確認
- Grafana dashboard 的 SQL 由 view 統一封裝（DB 層）還是寫在 dashboard JSON 裡（dashboard 層）？傾向 view 層封裝以便重用，但會綁定資料庫類型，待 Stage 2 評估

