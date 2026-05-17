<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Concerns\HasUlids;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;
use Illuminate\Support\Collection;

/**
 * @property string $id
 * @property string $slug
 * @property string|null $description
 */
#[Fillable(['slug', 'description'])]
class Group extends Model
{
    use HasUlids;

    public const string SLUG_PATTERN = '/^[a-z0-9-]+$/';

    protected $table = 'dp_groups';

    /**
     * @return BelongsToMany<Repo, $this>
     */
    public function repos(): BelongsToMany
    {
        return $this->belongsToMany(Repo::class, 'dp_groups_repos');
    }

    /**
     * @return BelongsToMany<Member, $this>
     */
    public function members(): BelongsToMany
    {
        return $this->belongsToMany(Member::class, 'dp_groups_members');
    }

    /**
     * @return Collection<int|string, mixed>
     */
    public function repoIds(): Collection
    {
        return $this->repos()->pluck('dp_repos.id');
    }
}
