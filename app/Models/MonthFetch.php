<?php

declare(strict_types=1);

namespace App\Models;

use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;

/**
 * @property int $id
 * @property int $repo_id
 * @property string $dataset
 * @property string $month
 * @property string $status
 * @property CarbonImmutable $fetched_at
 */
#[Fillable(['repo_id', 'dataset', 'month', 'status', 'fetched_at'])]
class MonthFetch extends Model
{
    public const DATASET_BUILDS = 'builds';
    public const DATASET_PULL_REQUESTS = 'pull_requests';
    public const STATUS_COMPLETE = 'complete';
    public const STATUS_PARTIAL = 'partial';

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'fetched_at' => 'immutable_datetime',
        ];
    }
}
