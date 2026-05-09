## ADDED Requirements

### Requirement: 儀表板 controller

系統 SHALL 提供 `App\Http\Controllers\Web\DashboardController`，由 `routes/web.php` 路由 `/dashboard` 觸發，從 PostgreSQL 取得 PR Lifecycle p90 與每日成功 build 平均時間兩組指標，並透過 Inertia 傳給前端。

#### Scenario: GET /dashboard 取得指標

- **WHEN** 使用者訪問 `/dashboard`
- **THEN** controller SHALL 查詢最近 30 天的兩組指標（PR Lifecycle p90 與 Build 平均時間），並 `Inertia::render('Dashboard', ['series' => [...], 'builds' => [...], 'range' => [...]])` 將資料當 props 傳入 Vue 頁面

#### Scenario: 查詢失敗

- **WHEN** 資料庫連線失敗
- **THEN** Laravel SHALL 拋出 exception 並交由既有 exception handler 處理（顯示 500 頁面或錯誤訊息），controller 不自行吞例外

### Requirement: 資料來源重用

系統 SHALL 重用既有 PostgreSQL schema/view 取得 PR Lifecycle 指標，不重新實作聚合邏輯。

#### Scenario: 直接讀既有 view

- **WHEN** controller 取資料
- **THEN** SHALL 透過 Laravel query builder 或 Eloquent 模型，從既有的 PR Lifecycle 相關 view/表查出每日 p90 值

### Requirement: PR Lifecycle 資料 payload 結構

Inertia 傳給前端的 `series` props SHALL 為陣列，元素含 `{ date: ISO8601 字串, pickup_p90_seconds: number|null, approval_p90_seconds: number|null, merge_p90_seconds: number|null }`。

#### Scenario: 缺日資料

- **WHEN** 某日無 PR 紀錄
- **THEN** 該日對應陣列元素的三個 p90 欄位 SHALL 為 `null`，前端負責繪圖時跳過該點

### Requirement: Build 每日平均時間查詢

系統 SHALL 提供查詢，計算指定區間內每日「成功 build」的平均執行時間。「成功 build」定義為 `is_failure = false` 且 `duration_seconds` 不為 null。

#### Scenario: 計算每日平均

- **WHEN** controller 對某區間呼叫 Build 平均時間查詢
- **THEN** 查詢 SHALL 對每一日返回該日所有成功 build 的 `duration_seconds` 算術平均值（單位：秒）

#### Scenario: 該日無成功 build

- **WHEN** 某日完全無成功 build
- **THEN** 該日 `avg_duration_seconds` SHALL 為 `null`，且 `successful_build_count` 為 `0`

### Requirement: Build 資料 payload 結構

Inertia 傳給前端的 `builds` props SHALL 為陣列，元素含 `{ date: ISO8601 字串, avg_duration_seconds: number|null, successful_build_count: integer }`。

#### Scenario: 完整序列

- **WHEN** controller 取得區間內每日 build 平均時間
- **THEN** 回傳陣列 SHALL 涵蓋區間的每一日（含無資料日，欄位為 `null` / `0`），順序由舊到新
