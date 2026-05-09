## 1. PHP 端整合 Inertia

- [x] 1.1 `composer require inertiajs/inertia-laravel`
- [x] 1.2 發佈並註冊 `HandleInertiaRequests` middleware（加入 `web` middleware group）
- [x] 1.3 將 `resources/views/app.blade.php` 改為 Inertia 根模板（含 `@viteReactRefresh` 不需要、僅留 `@vite(['resources/js/app.ts'])` 與 `@inertia`）
- [x] 1.4 確認 `routes/web.php` 預設路由由 Inertia 接管或保留 fallback

## 2. JS 端整合 Vue 3 + Inertia

- [x] 2.1 加入依賴：`vue`、`@inertiajs/vue3`、`@vitejs/plugin-vue`、`echarts`、`vue-echarts`
- [x] 2.2 加入開發依賴：`typescript`、`vue-tsc`、`@types/node`
- [x] 2.3 於 repo 根新增 `tsconfig.json`（vue strict 推薦設定）
- [x] 2.4 修改 `vite.config.js`：加入 `@vitejs/plugin-vue`、`resolve.extensions` 含 `.ts`、`.vue`
- [x] 2.5 將 `resources/js/app.js` 重命名為 `app.ts`，使用 `createInertiaApp`、`resolvePageComponent`，掛上 Vue app
- [x] 2.6 新增 `resources/js/types/inertia.d.ts` 與 `resources/js/types/metrics.ts`

## 3. 前端骨架與元件

- [x] 3.1 建立 `resources/js/Layouts/AppLayout.vue`：頁首（標題、簡單導覽）+ slot
- [x] 3.2 建立 `resources/js/Components/EChart.vue`：props 接 `option`，內部 `useECharts`，響應視窗 resize、option 變動、卸載釋放
- [x] 3.3 套用 Tailwind 撰寫 layout 與頁面樣式（不引入 UI kit）

## 4. Dashboard 頁面與 controller

- [x] 4.1 新增 `app/Http/Controllers/Web/DashboardController.php`，`index` 方法回傳 `Inertia::render('Dashboard', [...])`
- [x] 4.2 在 `routes/web.php` 註冊 `Route::get('/dashboard', [DashboardController::class, 'index'])`
- [x] 4.3 撰寫 PR Lifecycle 查詢：取最近 30 天每日 Pickup/Approval/Merge p90（`PrLifecycleQuery`）
- [x] 4.4 建立 `resources/js/Pages/Dashboard.vue`，包入 `AppLayout`，把 `series` props 轉成 ECharts option，渲染折線圖
- [x] 4.5 處理空資料 UI（PR Lifecycle 與 Build 區塊各自獨立顯示「無資料」訊息）

## 4b. Build 平均時間圖

- [x] 4b.1 新增 `app/Services/Web/BuildDurationQuery.php`：每日成功 build (`is_failure=false` 且 `duration_seconds` 非 null) 的平均時間（秒）
- [x] 4b.2 `DashboardController` 注入 `BuildDurationQuery`，傳 `builds` props
- [x] 4b.3 `resources/js/types/metrics.ts` 新增 `BuildDurationPoint` 與 `DashboardProps.builds`
- [x] 4b.4 `Pages/Dashboard.vue` 新增第二張面積折線圖（單位分鐘），tooltip 顯示「平均：X 分鐘 / 成功 build：Y 筆」

## 5. 驗證與部署

- [x] 5.1 `npm run build` 通過、`vue-tsc --noEmit` 無 type error
- [x] 5.2 `php artisan serve` + `npm run dev` 開 `/dashboard` 已透過 Chrome 外掛驗證：兩張圖正常渲染、無 console error。PR Lifecycle 圖只見 Pickup p90 一條線（Approval/Merge 為資料本身缺值，與 Web UI 無關）；Build 平均時間圖在 04-09~04-30 區間正常顯示，平均值 5–12 分鐘
- [ ] 5.3 production：`npm run build` 後以 Laravel container 啟動，訪問 `/dashboard` 正常（待使用者驗證）
- [x] 5.4 確認 Grafana dashboard 不受影響，既有 Laravel CLI 仍正常（route:list 通過、未觸動 grafana 設定）
- [x] 5.5 README 加上「Web UI（PoC）」段落，說明啟動方式
