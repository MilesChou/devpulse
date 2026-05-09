<?php

declare(strict_types=1);

return [

    /*
    |--------------------------------------------------------------------------
    | External API tokens
    |--------------------------------------------------------------------------
    |
    | 從環境變數讀取，避免硬編碼導致 commit 進 repo。詳見 tool-configuration spec。
    |
    */

    'github_token' => env('GITHUB_TOKEN'),
    'travis_token' => env('TRAVIS_TOKEN'),

    /*
    |--------------------------------------------------------------------------
    | Excluded bots
    |--------------------------------------------------------------------------
    |
    | 開的 PR、留的 review 都會被排除在統計外。值對應 GitHub account。
    | 後續可在 group 層級擴充覆寫機制（暫未實作）。
    |
    */

    'excluded_bots' => [
        'dependabot[bot]',
        'dependabot-preview[bot]',
        'renovate[bot]',
        'github-actions[bot]',
        'copilot-pull-request-reviewer[bot]',
    ],

    /*
    |--------------------------------------------------------------------------
    | PR size buckets
    |--------------------------------------------------------------------------
    |
    | 以 PR 改動的「總行數（additions + deletions）」分桶。
    | 鍵為桶名，值為「上限（不含）」；最後一個桶的上限為 null 代表無上限。
    | 使用者可在 group 層覆寫。
    |
    */

    'pr_size_buckets' => [
        'XS' => 50,
        'S' => 200,
        'M' => 500,
        'L' => 1000,
        'XL' => null,
    ],

    /*
    |--------------------------------------------------------------------------
    | Dashboard 觀測窗
    |--------------------------------------------------------------------------
    |
    | dashboard 預設往回看幾天（含當天）。使用者可透過 query string
    | `?days=14` 覆寫。
    |
    */

    'dashboard_days' => env('DEVPULSE_DASHBOARD_DAYS', 30),

    /*
    |--------------------------------------------------------------------------
    | 預警閾值 fallback
    |--------------------------------------------------------------------------
    |
    | 觀測指標的預警閾值 fallback。將來會在 group 層加上 `thresholds` 欄位、
    | 由特定 group 的設定覆寫；此處的值作為 fallback 使用。使用者亦可透過
    | query string 在執行時暫時覆寫，例：/dashboard?error_threshold=0.25。
    |
    */

    'thresholds' => [
        // 錯誤率超過此值視為警示（0.0–1.0）
        'error_rate' => env('DEVPULSE_ERROR_THRESHOLD', 0.30),

        // 重推（builds/PRs）超過此值視為警示
        'iteration' => env('DEVPULSE_ITERATION_THRESHOLD', 3.0),
    ],

    /*
    |--------------------------------------------------------------------------
    | human_signals 建議分類
    |--------------------------------------------------------------------------
    |
    | repo 的 human_signals.category 並無強制 enum 限制（VO 接受任何非空字串），
    | 此處列出後台下拉預設的常見分類，方便輸入；自訂值也能存進 DB。
    |
    */

    'human_signal_categories' => [
        'lint',
        'test',
        'build',
        'format',
        'syntax',
        'other',
    ],

];
