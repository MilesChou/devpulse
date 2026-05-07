<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use App\Models\Group;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;

#[Signature('devpulse:group:create {slug : 識別字（小寫英數與 dash，例如 my-team）} {--description= : 給人看的描述（選填）}')]
#[Description('建立一個 group（觀測群體）')]
class GroupCreateCommand extends Command
{
    private const string SLUG_PATTERN = '/^[a-z0-9-]+$/';

    public function handle(): int
    {
        $slug = (string)$this->argument('slug');
        $description = $this->option('description');
        $description = is_string($description) ? $description : null;

        if (preg_match(self::SLUG_PATTERN, $slug) !== 1) {
            $this->error("slug 格式錯誤：只允許小寫英數與 dash（例如 my-team）");

            return self::FAILURE;
        }

        if (Group::query()->where('slug', $slug)->exists()) {
            $this->error("slug `{$slug}` 已存在");

            return self::FAILURE;
        }

        $group = Group::query()->create([
            'slug' => $slug,
            'description' => $description,
        ]);

        $this->info("已建立 group #{$group->id}：{$group->slug}");

        return self::SUCCESS;
    }
}
