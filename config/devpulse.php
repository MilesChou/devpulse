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

];
