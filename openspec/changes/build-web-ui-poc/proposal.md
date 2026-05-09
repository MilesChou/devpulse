## Why

devpulse 目前只有 Grafana dashboard 一種呈現層，未來想做一個專屬 Web UI 以支援自訂報表、匯出、分享等場景。在投入完整實作前，先建立一個最小可行 PoC，驗證「Laravel + Inertia.js + Vue 3 + ECharts」的端到端鏈路可行，並建立後續演進的骨架。本 change 僅做 PoC，不含登入、匯出、排程、自訂報表等功能。

## What Changes

- 新增 Inertia.js + Vue 3 前端骨架，整合進既有 Laravel 應用：
  - 既有 `vite.config.js` 加入 `@vitejs/plugin-vue`
  - `resources/js/app.ts` 改為 Inertia 入口，掛上根元件
  - `resources/views/app.blade.php` 改為 Inertia 根模板
- 新增伺服器端：Laravel route + controller 直接以 Eloquent / query builder 從 PostgreSQL 撈 PR Lifecycle 指標，透過 Inertia `render` 把資料傳給 Vue 頁面
- 新增 `/dashboard` 預設儀表板頁：顯示兩張 ECharts 圖
  - PR Lifecycle p90 折線圖（三條線：Pickup / Approval / Merge）
  - 每日成功 build 平均時間面積折線圖（單位分鐘，tooltip 含當日成功 build 筆數）
- 樣式採 Tailwind CSS（既有 `@tailwindcss/vite` 已安裝）
- **非目標**：SSR、OAuth/登入、自訂報表、CSV/PDF 匯出、URL 分享狀態、排程寄送、命名報表（皆於後續 change 處理）

## Capabilities

### New Capabilities

- `web-ui`: Inertia + Vue 3 應用骨架（含 layout、預設儀表板頁、ECharts 圖表元件、Tailwind 樣式）
- `web-data-access`: Laravel controller 從 PostgreSQL 唯讀取得 PR Lifecycle 與每日成功 Build 平均時間兩組指標，透過 Inertia 傳遞給前端

### Modified Capabilities

（無——既有 capability 的 spec 不變）

## Impact

- **程式碼**：新增 `resources/js/Pages/`、`resources/js/Components/`、`resources/js/Layouts/` 與相關 `.vue` 檔；`resources/js/app.ts`、`resources/views/app.blade.php`、`vite.config.js` 修改；`routes/web.php` 新增 dashboard 路由；新增 `app/Http/Controllers/Web/DashboardController.php`
- **依賴**（PHP）：新增 `inertiajs/inertia-laravel`
- **依賴**（JS）：新增 `vue@^3`、`@inertiajs/vue3`、`@vitejs/plugin-vue`、`vue-tsc`、`typescript`、`echarts`、`vue-echarts`
- **資料庫**：共用既有 write DB user 連線（PoC 階段不另建 read-only role）；不新增表
- **設定**：`tsconfig.json` 新增於 repo 根；`vite.config.js` 加 vue plugin
- **部署**：不新增 Docker service；既有 Laravel container 同時提供 Web UI；前端 build 由既有 `npm run build` 完成
- **不影響**：Grafana dashboard、Laravel CLI/抓資料流程、既有 blade 視圖（若有）
