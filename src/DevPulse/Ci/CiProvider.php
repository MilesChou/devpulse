<?php

declare(strict_types=1);

namespace DevPulse\Ci;

use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;

interface CiProvider
{
    /**
     * 列出指定 repo 在指定月份內的所有 build。
     *
     * 回傳 iterable 以允許 generator / lazy pagination；caller 可 foreach 一次性消費。
     *
     * @return iterable<Build>
     */
    public function listBuildsInMonth(RepoFullName $repoFullName, MonthRange $month): iterable;

    /**
     * 取得單一 build 的完整 log 文字（用於失敗分類）。
     */
    public function getBuildLog(RepoFullName $repoFullName, string $externalBuildId): string;
}
