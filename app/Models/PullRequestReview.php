<?php

declare(strict_types=1);

namespace App\Models;

use DevPulse\Vcs\ReviewState;
use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * @property int $id
 * @property int $pull_request_id
 * @property string $reviewer_account
 * @property ReviewState $state
 * @property CarbonImmutable $submitted_at
 */
#[Fillable([
    'pull_request_id',
    'reviewer_account',
    'state',
    'submitted_at',
])]
class PullRequestReview extends Model
{
    /**
     * @return BelongsTo<PullRequest, $this>
     */
    public function pullRequest(): BelongsTo
    {
        return $this->belongsTo(PullRequest::class);
    }

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'state' => ReviewState::class,
            'submitted_at' => 'immutable_datetime',
        ];
    }
}
