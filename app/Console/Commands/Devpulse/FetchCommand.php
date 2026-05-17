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
    . ' {repo : repo full name (e.g. owner/name)}',
)]
#[Description(
    'Dispatch a job to fetch all historical PRs for the given repo (state=all),'
    . ' then dispatch one enrichment job per PR',
)]
class FetchCommand extends Command
{
    public function handle(): int
    {
        $repoFullName = (string)$this->argument('repo');

        $repo = Repo::query()->where('name', $repoFullName)->first();
        if ($repo === null) {
            $this->error("Repo `$repoFullName` not found. Add it first with devpulse:repo:add");

            return self::FAILURE;
        }

        FetchAllPullRequestsJob::dispatch($repo->id);

        $this->info("Fetch job dispatched for: $repoFullName");

        return self::SUCCESS;
    }
}
