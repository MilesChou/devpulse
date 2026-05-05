<?php

declare(strict_types=1);

namespace App\Persistence\Repository;

use App\Domain\Vcs\PullRequestSummary;
use App\Models\PullRequest;
use App\Persistence\Mapper\PullRequestMapper;

final class PullRequestRepository
{
    public function __construct(private readonly PullRequestMapper $mapper)
    {
    }

    /**
     * @param iterable<PullRequestSummary> $pulls
     * @param array<int, array<string, mixed>> $rawPayloads number => raw payload
     */
    public function upsertMany(int $repoId, iterable $pulls, array $rawPayloads = []): int
    {
        $count = 0;
        foreach ($pulls as $vo) {
            $payload = $rawPayloads[$vo->number] ?? [];
            $attributes = $this->mapper->toAttributes($vo, $repoId, $payload);

            PullRequest::query()->updateOrCreate(
                ['repo_id' => $repoId, 'number' => $vo->number],
                $attributes,
            );
            $count++;
        }

        return $count;
    }
}
