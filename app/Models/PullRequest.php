<?php

declare(strict_types=1);

namespace App\Models;

use DevPulse\Vcs\Platform;
use DevPulse\Vcs\PullRequestStatus;
use Carbon\CarbonImmutable;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Concerns\HasUlids;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * @property string $id
 * @property Platform $platform
 * @property string $repo_id
 * @property int $number PR 編號
 * @property string $author_account 作者帳號
 * @property PullRequestStatus $status
 * @property int $additions 新增行數
 * @property int $deletions 刪除行數
 * @property int $total_changed_lines 總變更行數（去正規化，Source of truth 在 VO）
 * @property string|null $size_bucket 大小分桶（去正規化，供 Grafana / SQL view 使用）
 * @property bool $is_draft 是否為草稿（去正規化，供 Grafana / SQL view 使用）
 * @property CarbonImmutable $pr_created_at PR 建立時間
 * @property CarbonImmutable|null $ready_at 轉為 ready for review 的時間
 * @property CarbonImmutable|null $first_review_at 第一筆任意 state 的 review 時間（Pickup Time 終點），僅計 ready_at 之後
 * @property CarbonImmutable|null $first_approved_at 第一筆 approved state 的 review 時間，僅計 ready_at 之後
 * @property int|null $time_to_approval 從 ready_at 到 first_approved_at 的秒數（去正規化）
 * @property int|null $time_to_merge 從 first_approved_at 到 merged_at 的秒數（去正規化）
 * @property CarbonImmutable|null $merged_at 合併時間
 * @property CarbonImmutable|null $closed_at 關閉時間
 */
#[Fillable([
    'id',
    'platform',
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
])]
class PullRequest extends Model
{
    use HasUlids;

    protected $table = 'dp_pull_requests';

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
            'platform' => Platform::class,
            'status' => PullRequestStatus::class,
            'is_draft' => 'boolean',
            'pr_created_at' => 'immutable_datetime',
            'ready_at' => 'immutable_datetime',
            'first_review_at' => 'immutable_datetime',
            'first_approved_at' => 'immutable_datetime',
            'merged_at' => 'immutable_datetime',
            'closed_at' => 'immutable_datetime',
        ];
    }
}
