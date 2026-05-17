<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Attributes\Fillable;
use Illuminate\Database\Eloquent\Concerns\HasUlids;
use Illuminate\Database\Eloquent\Model;

/**
 * @property string $id
 * @property string $slug
 * @property string $name
 * @property string $type
 * @property string $url
 * @property list<array{category: string, pattern: string}> $human_signals
 */
#[Fillable(['slug', 'name', 'type', 'url', 'human_signals'])]
class Repo extends Model
{
    use HasUlids;

    protected $table = 'dp_repos';

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
