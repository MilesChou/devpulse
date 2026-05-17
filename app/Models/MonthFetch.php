<?php

declare(strict_types=1);

namespace App\Models;

use App\Persistence\Enum\Dataset;
use App\Persistence\Enum\MonthFetchStatus;
use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Concerns\HasUlids;
use Illuminate\Database\Eloquent\Model;

/**
 * @property string $id
 * @property string $repo_id
 * @property Dataset $dataset
 * @property string $month
 * @property MonthFetchStatus $status
 * @property CarbonImmutable $fetched_at
 */
#[Fillable(['repo_id', 'dataset', 'month', 'status', 'fetched_at'])]
class MonthFetch extends Model
{
    use HasUlids;

    protected $table = 'dp_month_fetches';

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
