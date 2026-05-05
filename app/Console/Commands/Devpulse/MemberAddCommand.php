<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use App\Models\Group;
use App\Models\Member;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;

#[Signature('devpulse:member:add {group : group slug} {github_account} {display_name}')]
#[Description('把成員加進指定 group（成員不存在則自動建立）')]
class MemberAddCommand extends Command
{
    public function handle(): int
    {
        $groupSlug = (string)$this->argument('group');
        $githubAccount = (string)$this->argument('github_account');
        $displayName = (string)$this->argument('display_name');

        $group = Group::query()->where('slug', $groupSlug)->first();
        if ($group === null) {
            $this->error("group `{$groupSlug}` 不存在，請先用 devpulse:group:create 建立");

            return self::FAILURE;
        }

        $member = Member::query()->firstOrCreate(
            ['github_account' => $githubAccount],
            ['display_name' => $displayName],
        );

        if ($group->members()->where('member_id', $member->id)->exists()) {
            $this->warn("成員 `{$githubAccount}` 已在 group `{$groupSlug}` 中");

            return self::SUCCESS;
        }

        $group->members()->attach($member);

        $this->info("已將 {$member->display_name}（{$member->github_account}）加進 group `{$groupSlug}`");

        return self::SUCCESS;
    }
}
