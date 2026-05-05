<?php

declare(strict_types=1);

namespace App\Domain\Ci;

use App\Domain\Shared\MonthRange;
use App\Domain\Shared\RepoFullName;

interface CiProvider
{
    /**
     * 列出指定 repo 在指定月份內的所有 build。
     *
     * 回傳 iterable 以允許 generator / lazy pagination；caller 可 foreach 一次性消費。
     *
     * @return iterable<BuildSummary>
     */
    public function listBuildsInMonth(RepoFullName $repoFullName, MonthRange $month): iterable;

    /**
     * 取得單一 build 的完整 log 文字（用於失敗分類）。
     */
    public function getBuildLog(RepoFullName $repoFullName, string $externalBuildId): string;
}
