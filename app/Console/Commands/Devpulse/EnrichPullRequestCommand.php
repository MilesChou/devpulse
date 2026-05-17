<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use App\Fetching\FetchOrchestrator;
use App\Models\Repo;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;

#[Signature(
    'devpulse:pr:enrich'
    . ' {repo : repo full name（例如 owner/name）}'
    . ' {number : PR 號碼}',
)]
#[Description('對單一 PR 強制重跑 enrich（detail + reviews），忽略 size_bucket 狀態')]
class EnrichPullRequestCommand extends Command
{
    public function handle(FetchOrchestrator $orchestrator): int
    {
        $repoFullName = (string)$this->argument('repo');
        $prNumber = (int)$this->argument('number');

        $repo = Repo::query()->where('name', $repoFullName)->first();
        if ($repo === null) {
            $this->error("repo `$repoFullName` 不存在");

            return self::FAILURE;
        }

        $this->info("重跑 enrich：{$repoFullName} #$prNumber");

        $found = $orchestrator->enrichOnePullRequestByNumber($repo, $prNumber);
        if (!$found) {
            $this->error("PR #$prNumber 在 DB 中不存在，請先執行 devpulse:fetch");

            return self::FAILURE;
        }

        $this->info('完成');

        return self::SUCCESS;
    }
}
