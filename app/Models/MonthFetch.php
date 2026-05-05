<?php

declare(strict_types=1);

namespace App\Models;

use App\Persistence\Enum\Dataset;
use App\Persistence\Enum\MonthFetchStatus;
use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;

/**
 * @property int $id
 * @property int $repo_id
 * @property Dataset $dataset
 * @property string $month
 * @property MonthFetchStatus $status
 * @property CarbonImmutable $fetched_at
 */
#[Fillable(['repo_id', 'dataset', 'month', 'status', 'fetched_at'])]
class MonthFetch extends Model
{
    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'dataset' => Dataset::class,
            'status' => MonthFetchStatus::class,
            'fetched_at' => 'immutable_datetime',
        ];
    }
}
