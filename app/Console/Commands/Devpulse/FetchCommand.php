<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use DevPulse\Shared\MonthRange;
use App\Fetching\FetchOrchestrator;
use App\Models\Repo;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use InvalidArgumentException;

#[Signature(
    'devpulse:fetch'
    . ' {repo : repo full name（例如 owner/name）}'
    . ' {month : Y-m 格式（例如 2026-04）}',
)]
#[Description('撈指定 repo 在指定月份的 builds + pull requests，寫入 DB')]
class FetchCommand extends Command
{
    public function handle(FetchOrchestrator $orchestrator): int
    {
        $repoFullName = (string)$this->argument('repo');
        $monthRaw = (string)$this->argument('month');

        try {
            $month = MonthRange::fromString($monthRaw);
        } catch (InvalidArgumentException $e) {
            $this->error("month 格式錯誤：{$e->getMessage()}（預期 Y-m，例如 2026-04）");

            return self::FAILURE;
        }

        $repo = Repo::query()->where('name', $repoFullName)->first();
        if ($repo === null) {
            $this->error("repo `$repoFullName` 不存在，請先用 devpulse:repo:add 新增");

            return self::FAILURE;
        }

        $this->info("撈取 $repoFullName / {$monthRaw}");

        $outcome = $orchestrator->fetch($repo, $month);

        if ($outcome->error !== null) {
            $this->error("  ✗ {$outcome->repoFullName}: {$outcome->error}");

            return self::FAILURE;
        }

        $this->info(sprintf(
            '  ✓ %s：builds=%d、prs=%d',
            $outcome->repoFullName,
            $outcome->buildsWritten,
            $outcome->pullRequestsWritten,
        ));

        return self::SUCCESS;
    }
}
