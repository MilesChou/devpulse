<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;
use Illuminate\Support\Collection;

/**
 * @property int $id
 * @property string $slug
 * @property string|null $description
 */
#[Fillable(['slug', 'description'])]
class Group extends Model
{
    public const string SLUG_PATTERN = '/^[a-z0-9-]+$/';

    /**
     * @return BelongsToMany<Repo, $this>
     */
    public function repos(): BelongsToMany
    {
        return $this->belongsToMany(Repo::class, 'group_repos');
    }

    /**
     * @return BelongsToMany<Member, $this>
     */
    public function members(): BelongsToMany
    {
        return $this->belongsToMany(Member::class, 'group_members');
    }

    /**
     * @return Collection<int|string, mixed>
     */
    public function repoIds(): Collection
    {
        return $this->repos()->pluck('repos.id');
    }
}
