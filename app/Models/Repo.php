<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;

/**
 * @property int $id
 * @property string $full_name
 * @property list<array{category: string, pattern: string}> $human_signals
 */
#[Fillable(['full_name', 'human_signals'])]
class Repo extends Model
{
    /**
     * @return BelongsToMany<Group, $this>
     */
    public function groups(): BelongsToMany
    {
        return $this->belongsToMany(Group::class, 'group_repos');
    }

    /**
     * @return array<string, string>
     */
    protected function casts(): array
    {
        return [
            'human_signals' => 'array',
        ];
    }
}
