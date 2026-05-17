<?php

declare(strict_types=1);

namespace Tests\Support;

use App\Models\Repo;
use Illuminate\Support\Str;

trait CreatesRepoModel
{
    /**
     * @param array<string, mixed> $overrides
     */
    protected function makeRepo(string $name = 'your-org/your-repo', array $overrides = []): Repo
    {
        return Repo::create(array_merge([
            'slug' => Str::slug(str_replace('/', '-', $name)) . '-' . Str::random(6),
            'name' => $name,
            'type' => 'github',
            'url' => "git@github.com:{$name}.git",
        ], $overrides));
    }
}
