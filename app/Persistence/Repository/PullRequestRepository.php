<?php

declare(strict_types=1);

namespace App\Persistence\Repository;

use App\Domain\Vcs\PullRequest;
use App\Models\PullRequest as PullRequestModel;
use App\Persistence\Mapper\PullRequestMapper;

final class PullRequestRepository
{
    public function __construct(private readonly PullRequestMapper $mapper)
    {
    }

    /**
     * @param iterable<PullRequest> $pulls
     */
    public function upsertMany(iterable $pulls): int
    {
        $count = 0;
        foreach ($pulls as $vo) {
            PullRequestModel::updateOrCreate(
                ['repo_id' => $vo->repoId->toInt(), 'number' => $vo->number->toInt()],
                $this->mapper->toAttributes($vo),
            );
            $count++;
        }

        return $count;
    }
}
