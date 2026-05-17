<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use DevPulse\Shared\RepoFullName;
use App\Models\Group;
use App\Models\Repo;
use InvalidArgumentException;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use Illuminate\Support\Str;

#[Signature(
    'devpulse:repo:add'
    . ' {group : group slug}'
    . ' {name : owner/name}'
    . ' {--type=github : github 或 gitlab}'
    . ' {--slug= : 短代號（預設由 name 推導，例如 owner/repo → owner-repo）}'
    . ' {--url= : git clone URL（預設由 type + name 推導）}',
)]
#[Description('把 repo 加進指定 group（repo 不存在則自動建立）')]
class RepoAddCommand extends Command
{
    public function handle(): int
    {
        $groupSlug = (string)$this->argument('group');
        $name = (string)$this->argument('name');
        $type = (string)$this->option('type');

        $group = Group::query()->where('slug', $groupSlug)->first();
        if ($group === null) {
            $this->error("group `{$groupSlug}` 不存在，請先用 devpulse:group:create 建立");

            return self::FAILURE;
        }

        try {
            new RepoFullName($name);
        } catch (InvalidArgumentException) {
            $this->error("name 格式錯誤：必須是 owner/name（例如 your-org/your-repo）");

            return self::FAILURE;
        }

        $slug = (string)($this->option('slug') ?: Str::slug(str_replace('/', '-', $name)));
        $url = (string)($this->option('url') ?: $this->defaultUrl($type, $name));

        $repo = Repo::query()->firstOrCreate(
            ['type' => $type, 'name' => $name],
            ['slug' => $slug, 'url' => $url],
        );

        if ($group->repos()->where('repo_id', $repo->id)->exists()) {
            $this->warn("repo `{$name}` 已在 group `{$groupSlug}` 中");

            return self::SUCCESS;
        }

        $group->repos()->attach($repo);

        $this->info("已將 {$repo->name} 加進 group `{$groupSlug}`");

        return self::SUCCESS;
    }

    private function defaultUrl(string $type, string $name): string
    {
        return match ($type) {
            'gitlab' => "git@gitlab.com:{$name}.git",
            default => "git@github.com:{$name}.git",
        };
    }
}
