## Context

devpulse 目前的呈現層是 Grafana dashboard。本 change 是 Web UI 系列的第一步——PoC，目標是用最少的工程量驗證「以 Laravel 為後端、Inertia + Vue 為前端」的鏈路能跑通端到端，並把骨架建好讓後續 change 在上面長。Laravel + Inertia 路線可重用既有 Laravel 框架（route、middleware、ORM、設定、container、Vite 整合都已就位），無需另起 Node 服務。

**繼承自既有專案的條件：**
- Laravel 13 已建立完整骨架；`package.json` 已有 Vite + Tailwind v4 + Laravel Vite plugin
- PostgreSQL schema 由 Laravel migration 維護，含 PR Lifecycle 相關 view/表
- Docker Compose 已可拉起 Laravel + Postgres + Grafana

**主要約束：**
- PoC 不投入 SSR、認證、匯出、排程；維持工程量最小
- 不引入 Node 後端服務；前端透過 Inertia 由 Laravel 直接 render

## Goals / Non-Goals

**Goals：**
- 將 Inertia + Vue 3 接進既有 Laravel 應用，不另起服務
- 提供 `/dashboard` 預設儀表板頁，顯示一張 ECharts 折線圖（PR Lifecycle p90）
- 建立 layout、頁面、元件、controller 的目錄慣例，供後續 change 沿用
- 確認 `npm run build` + Laravel 容器化可一次部署

**Non-Goals：**
- SSR（Inertia SSR sub-process）
- 登入、權限、白名單
- 自訂報表、匯出（CSV/PDF）、URL 分享狀態、排程寄送、命名報表
- 行動裝置最佳化、深色模式（不排斥但不列入 PoC 驗收標準）
- 引入 UI 元件庫（PrimeVue、Element 等），純 Tailwind 即可

## Decisions

### Decision 1：Laravel + Inertia.js + Vue 3 作為技術棧

**選擇：** `inertiajs/inertia-laravel` + `@inertiajs/vue3` + Vue 3（`<script setup>` + TypeScript）+ Tailwind v4

**理由：**
- 重用既有 Laravel 全部能力（route、middleware、ORM、queue、auth 未來可加），無須維護第二個服務
- Inertia 把「Laravel route → Vue 頁面」連起來，免設計 REST API；前端可直接拿 props
- 與既有 `vite.config.js` 與 Tailwind 設定相容，最小新增

**替代方案：**
- Nuxt 3：要另起 Node 服務、要重做 DB 連線/設定/auth 中介層，PoC 階段過重
- Vite + Vue SPA + Laravel API：要設計 API 與處理 CORS、auth token，比 Inertia 麻煩
- 純 Blade + Alpine：自訂報表的互動需求未來會卡住，現在先選 Vue 較長線

### Decision 2：不開 Inertia SSR

**選擇：** PoC 階段不啟用 Inertia SSR

**理由：**
- SSR 需要 `php artisan inertia:start-ssr` 與 Node sub-process，部署 / Docker Compose 要動
- PoC 沒有「分享連結首屏 SEO」需求，CSR 已足夠
- 後續 change 若需要再加（Inertia 設計上 SSR 為 opt-in）

### Decision 3：DB 存取走 Laravel ORM / query builder

**選擇：** Web 用的指標 controller 直接用 Eloquent / query builder 查 PR Lifecycle view，與既有抓資料服務共用 Eloquent connection

**理由：**
- 不引入第二套 DB client（如 postgres.js）
- 共用 write user（PoC 階段，後續可換 read-only）
- 查詢若已有 service / repository，直接重用；沒有則就近建立

**替代方案：**
- 另建 read-only connection：未來再做（v1 之後），PoC 不必

### Decision 4：圖表用 ECharts via `vue-echarts`

**選擇：** Apache ECharts + `vue-echarts` 包裝；建立通用 `<EChart :option="..." />` 元件

**理由：**
- 已於前一輪確認；Vue 圖表生態最完整、license 寬鬆

### Decision 5：前端目錄慣例

**選擇：**

```
resources/
  js/
    app.ts                  # Inertia entry
    Pages/
      Dashboard.vue         # 對應 /dashboard
    Layouts/
      AppLayout.vue         # 全域版面
    Components/
      EChart.vue            # ECharts 包裝
    types/
      metrics.ts            # 共用型別
  views/
    app.blade.php           # Inertia 根模板
```

**理由：**
- 沿用 Inertia/Laravel 社群常見慣例（Pages/Layouts/Components 大寫資料夾）
- 後續 change 加新頁時直接在 `Pages/` 下對應 controller 即可

### Decision 6：Controller 採 thin controller + Inertia render

**選擇：** `App\Http\Controllers\Web\DashboardController@index` 取資料後 `Inertia::render('Dashboard', [...props])`；資料聚合若已有 service 則直接呼叫，不在 controller 內寫 SQL

**理由：**
- 維持與既有 Laravel 慣例一致（controller 薄）
- 後續報表頁照同模式擴充

## Risks / Trade-offs

- **[Vite plugin-vue 與既有 blade 編譯衝突] → Mitigation：** vue plugin 僅處理 `.vue`；blade 視圖維持原有編譯路徑；本 change 不刪既有 blade
- **[TypeScript 引入導致 build 變慢] → Mitigation：** 用 `vue-tsc --noEmit` 僅做型別檢查；Vite build 速度不受影響
- **[Inertia 把所有 props 序列化進 HTML] → Mitigation：** 圖表資料量小（30~90 個資料點），可接受；未來資料量大時改用 partial reload / lazy props
- **[共用 write DB user 風險] → Mitigation：** PoC 範圍小、SQL 由 Laravel ORM 統一管，不直接拼 raw SQL；v1 階段再切 read-only role

## Migration Plan

1. 安裝 `inertiajs/inertia-laravel`，發佈中介層 `HandleInertiaRequests`
2. 安裝 JS 依賴：`vue`、`@inertiajs/vue3`、`@vitejs/plugin-vue`、`vue-tsc`、`typescript`、`echarts`、`vue-echarts`
3. 將 `resources/js/app.js` 改為 `app.ts`，使用 `createInertiaApp`
4. `vite.config.js` 加入 vue plugin、設定 `resolve.extensions`
5. `resources/views/app.blade.php` 改為 Inertia 根模板（含 `@inertia` 與 `@vite`）
6. 新增 `routes/web.php` 的 `/dashboard` 與 `DashboardController`
7. 建立 `Pages/Dashboard.vue`、`Layouts/AppLayout.vue`、`Components/EChart.vue`
8. 跑 `npm run build` 與 `php artisan serve` 端到端驗證

**Rollback：** 純新增。Rollback 即移除新增檔案、回退 `app.js`/`app.blade.php`/`vite.config.js`、`composer remove inertiajs/inertia-laravel`。
