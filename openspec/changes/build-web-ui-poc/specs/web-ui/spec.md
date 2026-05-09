## ADDED Requirements

### Requirement: Inertia + Vue 3 應用骨架

系統 SHALL 在既有 Laravel 應用內整合 Inertia.js 與 Vue 3，並使用 `<script setup>` 搭配 TypeScript；前端入口為 `resources/js/app.ts`，根模板為 `resources/views/app.blade.php`。

#### Scenario: 啟動開發伺服器

- **WHEN** 開發者執行 `php artisan serve` 與 `npm run dev`
- **THEN** 瀏覽器訪問 `/dashboard` SHALL 看到 Inertia 渲染的 Vue 頁面，無 console error

#### Scenario: 產生 production build

- **WHEN** 開發者執行 `npm run build`
- **THEN** Vite SHALL 產出 `public/build/` 內的 manifest 與 assets，且 `php artisan serve` 訪問頁面 SHALL 正確載入打包後的 JS/CSS

### Requirement: 預設儀表板頁

系統 SHALL 提供 `/dashboard` 預設儀表板頁，依序呈現兩張圖：
1. PR Lifecycle p90（Pickup / Approval / Merge 三條線共用一張折線圖）
2. 每日成功 build 平均時間（面積折線圖，單位分鐘）

#### Scenario: 開啟儀表板

- **WHEN** 使用者訪問 `/dashboard`
- **THEN** 系統 SHALL 顯示「PR Lifecycle p90」折線圖（含 Pickup p90、Approval p90、Merge p90 三條線）與「每日成功 Build 平均時間」面積折線圖，X 軸皆為日期，預設區間為最近 30 天

#### Scenario: PR Lifecycle 無資料

- **WHEN** PR Lifecycle 後端查詢結果整個區間皆為 null
- **THEN** 該區塊 SHALL 顯示「無 PR Lifecycle 資料」提示，且不丟出前端錯誤；Build 區塊不受影響

#### Scenario: Build 無資料

- **WHEN** Build 後端查詢結果整個區間皆無成功 build
- **THEN** 該區塊 SHALL 顯示「無 Build 資料」提示，且不丟出前端錯誤；PR Lifecycle 區塊不受影響

#### Scenario: Build 圖 tooltip

- **WHEN** 使用者將游標停在 Build 圖某一日資料點
- **THEN** tooltip SHALL 顯示該日的平均時間（分鐘）與「成功 build 筆數」

### Requirement: ECharts 圖表元件

系統 SHALL 提供可重用的 `Components/EChart.vue` 元件，封裝 ECharts 初始化、option 更新、視窗 resize 自動重繪、卸載時釋放實例。

#### Scenario: option 變更時更新

- **WHEN** 父元件透過 props 傳入新的 `option`
- **THEN** 圖表 SHALL 套用新 option，無需手動 re-mount

#### Scenario: 視窗 resize

- **WHEN** 使用者調整瀏覽器視窗寬度
- **THEN** 圖表 SHALL 自動重新計算尺寸並重繪

### Requirement: 全域版面

系統 SHALL 提供 `Layouts/AppLayout.vue` 全域版面，包含頁首與內容區；所有頁面 SHALL 透過 `definePageLayout` 或元件包裹方式套用。

#### Scenario: 頁面套用 layout

- **WHEN** `Pages/Dashboard.vue` 載入
- **THEN** 頁面 SHALL 顯示在 `AppLayout` 之內，頁首固定可見
