<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use DevPulse\Shared\MonthRange;
use App\Fetching\FetchOrchestrator;
use App\Models\Group;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use InvalidArgumentException;

#[Signature(
    'devpulse:fetch'
    . ' {group : group slug}'
    . ' {month : Y-m 格式（例如 2026-04）}'
    . ' {--force : 即使該月已標記 complete 也重撈}',
)]
#[Description('撈指定 group 在指定月份的 builds + pull requests，寫入 DB')]
class FetchCommand extends Command
{
    public function handle(FetchOrchestrator $orchestrator): int
    {
        $groupSlug = (string)$this->argument('group');
        $monthRaw = (string)$this->argument('month');
        $force = (bool)$this->option('force');

        try {
            $month = MonthRange::fromString($monthRaw);
        } catch (InvalidArgumentException $e) {
            $this->error("month 格式錯誤：{$e->getMessage()}（預期 Y-m，例如 2026-04）");

            return self::FAILURE;
        }

        $group = Group::query()->where('slug', $groupSlug)->first();
        if ($group === null) {
            $this->error("group `$groupSlug` 不存在");

            return self::FAILURE;
        }

        $repoCount = $group->repos()->count();
        if ($repoCount === 0) {
            $this->warn("group `$groupSlug` 沒有任何 repo，請先用 devpulse:repo:add 加 repo");

            return self::SUCCESS;
        }

        $this->info("撈取 $groupSlug / {$monthRaw}（{$repoCount} repos）" . ($force ? '（force mode）' : ''));

        $result = $orchestrator->fetch($group, $month, $force);

        $hasError = false;
        foreach ($result->repos as $outcome) {
            if ($outcome->error !== null) {
                $this->error("  ✗ {$outcome->repoFullName}: {$outcome->error}");
                $hasError = true;
            } elseif ($outcome->skipped) {
                $this->line("  ↳ {$outcome->repoFullName}：已 complete，跳過（用 --force 重撈）");
            } else {
                $this->info(sprintf(
                    '  ✓ %s：builds=%d、prs=%d',
                    $outcome->repoFullName,
                    $outcome->buildsWritten,
                    $outcome->pullRequestsWritten,
                ));
            }
        }

        $this->line('');
        $this->info(sprintf(
            '總計：builds=%d、prs=%d、skipped=%d',
            $result->totalBuildsWritten(),
            $result->totalPullRequestsWritten(),
            $result->totalReposSkipped(),
        ));

        return $hasError ? self::FAILURE : self::SUCCESS;
    }
}
