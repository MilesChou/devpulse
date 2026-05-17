<?php

declare(strict_types=1);

namespace App\Models;

use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Concerns\HasUlids;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * @property string $id
 * @property string $repo_id
 * @property string $external_id
 * @property string $commit_sha
 * @property string|null $author_account
 * @property int|null $pr_number
 * @property string $status
 * @property string $trigger
 * @property string|null $branch
 * @property bool $is_post_merge
 * @property bool $is_pull_request
 * @property bool $is_deploy_event
 * @property bool $is_failure
 * @property CarbonImmutable $started_at
 * @property int|null $duration_seconds
 * @property array<string, mixed> $raw_payload
 * @property-read Repo $repo
 */
#[Fillable([
    'repo_id',
    'external_id',
    'commit_sha',
    'author_account',
    'pr_number',
    'status',
    'trigger',
    'branch',
    'is_post_merge',
    'is_pull_request',
    'is_deploy_event',
    'is_failure',
    'started_at',
    'duration_seconds',
    'raw_payload',
])]
class Build extends Model
{
    use HasUlids;

    protected $table = 'dp_builds';

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
            'status' => 'string',
            'is_post_merge' => 'boolean',
            'is_pull_request' => 'boolean',
            'is_deploy_event' => 'boolean',
            'is_failure' => 'boolean',
            'started_at' => 'immutable_datetime',
            'duration_seconds' => 'integer',
            'raw_payload' => 'array',
        ];
    }
}
