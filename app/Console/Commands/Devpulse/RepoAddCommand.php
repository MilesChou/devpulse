<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use App\Domain\Shared\RepoFullName;
use App\Models\Group;
use App\Models\Repo;
use InvalidArgumentException;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;

#[Signature('devpulse:repo:add {group} {full_name : owner/name}')]
#[Description('把 repo 加進指定 group（repo 不存在則自動建立）')]
class RepoAddCommand extends Command
{
    public function handle(): int
    {
        $groupSlug = (string)$this->argument('group');
        $fullName = (string)$this->argument('full_name');

        $group = Group::query()->where('slug', $groupSlug)->first();
        if ($group === null) {
            $this->error("group `{$groupSlug}` 不存在，請先用 devpulse:group:create 建立");

            return self::FAILURE;
        }

        try {
            new RepoFullName($fullName);
        } catch (InvalidArgumentException) {
            $this->error("full_name 格式錯誤：必須是 owner/name（例如 your-org/your-repo）");

            return self::FAILURE;
        }

        $repo = Repo::query()->firstOrCreate(['full_name' => $fullName]);

        if ($group->repos()->where('repo_id', $repo->id)->exists()) {
            $this->warn("repo `{$fullName}` 已在 group `{$groupSlug}` 中");

            return self::SUCCESS;
        }

        $group->repos()->attach($repo);

        $this->info("已將 {$repo->full_name} 加進 group `{$groupSlug}`");

        return self::SUCCESS;
    }
}
