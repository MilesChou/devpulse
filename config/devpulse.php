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

];
