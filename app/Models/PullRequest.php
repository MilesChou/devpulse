<?php

declare(strict_types=1);

namespace App\Models;

use App\Domain\Vcs\PullRequestStatus;
use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * @property int $id
 * @property int $repo_id
 * @property int $number
 * @property string $author_account
 * @property PullRequestStatus $status
 * @property int $additions
 * @property int $deletions
 * @property int $total_changed_lines
 * @property string|null $size_bucket
 * @property bool $is_draft
 * @property CarbonImmutable $pr_created_at
 * @property CarbonImmutable|null $ready_at
 * @property CarbonImmutable|null $first_review_at
 * @property CarbonImmutable|null $merged_at
 * @property CarbonImmutable|null $closed_at
 * @property array<string, mixed> $raw_payload
 */
#[Fillable([
    'repo_id',
    'number',
    'author_account',
    'status',
    'additions',
    'deletions',
    'total_changed_lines',
    'size_bucket',
    'is_draft',
    'pr_created_at',
    'ready_at',
    'first_review_at',
    'merged_at',
    'closed_at',
    'raw_payload',
])]
class PullRequest extends Model
{
    /**
     * @return BelongsTo<Repo, $this>
     */
    public function repo(): BelongsTo
    {
        return $this->belongsTo(Repo::class);
    }

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'status' => PullRequestStatus::class,
            'is_draft' => 'boolean',
            'pr_created_at' => 'immutable_datetime',
            'ready_at' => 'immutable_datetime',
            'first_review_at' => 'immutable_datetime',
            'merged_at' => 'immutable_datetime',
            'closed_at' => 'immutable_datetime',
            'raw_payload' => 'array',
        ];
    }
}
