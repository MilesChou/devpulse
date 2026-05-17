<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use App\Jobs\FetchAllPullRequestsJob;
use App\Models\Repo;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;

#[Signature(
    'devpulse:fetch'
    . ' {repo : repo full name（例如 owner/name）}',
)]
#[Description('Dispatch Job 抽取指定 repo 全部歷史 PR（state=all），完成後逐筆 dispatch enrichment')]
class FetchCommand extends Command
{
    public function handle(): int
    {
        $repoFullName = (string)$this->argument('repo');

        $repo = Repo::query()->where('name', $repoFullName)->first();
        if ($repo === null) {
            $this->error("repo `$repoFullName` 不存在，請先用 devpulse:repo:add 新增");

            return self::FAILURE;
        }

        FetchAllPullRequestsJob::dispatch($repo->id);

        $this->info("已 dispatch FetchAllPullRequestsJob：$repoFullName");

        return self::SUCCESS;
    }
}
