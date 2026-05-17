<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\Platform;
use App\Models\Repo;
use InvalidArgumentException;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use Illuminate\Support\Str;
use ValueError;

#[Signature(
    'devpulse:repo:add'
    . ' {name : owner/name}'
    . ' {--type=github : github 或 gitlab}'
    . ' {--slug= : 短代號（預設由 name 推導，例如 owner/repo → owner-repo）}'
    . ' {--url= : git clone URL（預設由 type + name 推導）}',
)]
#[Description('新增 repo（已存在則跳過）')]
class RepoAddCommand extends Command
{
    public function handle(): int
    {
        $name = (string)$this->argument('name');
        $typeRaw = (string)$this->option('type');

        try {
            new RepoFullName($name);
        } catch (InvalidArgumentException) {
            $this->error("name 格式錯誤：必須是 owner/name（例如 your-org/your-repo）");

            return self::FAILURE;
        }

        try {
            $platform = Platform::from($typeRaw);
        } catch (ValueError) {
            $this->error("type 不支援：{$typeRaw}（可用：github、gitlab）");

            return self::FAILURE;
        }

        $slug = (string)($this->option('slug') ?: Str::slug(str_replace('/', '-', $name)));
        $url = (string)($this->option('url') ?: $this->defaultUrl($platform, $name));

        $repo = Repo::query()->firstOrCreate(
            ['type' => $platform->value, 'name' => $name],
            ['slug' => $slug, 'url' => $url],
        );

        if ($repo->wasRecentlyCreated) {
            $this->info("已新增 repo {$repo->name}");
        } else {
            $this->line("repo `{$name}` 已存在");
        }

        return self::SUCCESS;
    }

    private function defaultUrl(Platform $platform, string $name): string
    {
        return match ($platform) {
            Platform::GitLab => "git@gitlab.com:{$name}.git",
            Platform::GitHub => "git@github.com:{$name}.git",
        };
    }
}
