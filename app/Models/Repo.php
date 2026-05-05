<?php

declare(strict_types=1);

namespace App\Models;

use App\Domain\Ci\CiProvider;
use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsToMany;

#[Fillable(['full_name', 'ci_provider'])]
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
            'ci_provider' => CiProvider::class,
        ];
    }
}
