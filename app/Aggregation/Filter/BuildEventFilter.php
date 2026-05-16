<?php

declare(strict_types=1);

namespace App\Aggregation\Filter;

use App\Models\Build;
use Illuminate\Database\Eloquent\Builder;

/**
 * 限制 Build query 只包含「個人責任範圍內」的 build。
 *
 * 預設排除 post-merge（is_post_merge=true）與 deploy event（is_deploy_event=true），
 * 避免這些失敗誤算為個人失敗率。
 */
final class BuildEventFilter
{
    public function __construct(
        private readonly bool $includePostMerge = false,
        private readonly bool $includeDeployEvents = false,
    ) {
    }

    /**
     * @param Builder<Build> $query
     * @return Builder<Build>
     */
    public function apply(Builder $query): Builder
    {
        if (! $this->includePostMerge) {
            $query->where('dp_builds.is_post_merge', false);
        }

        if (! $this->includeDeployEvents) {
            $query->where('dp_builds.is_deploy_event', false);
        }

        return $query;
    }
}
