<?php

declare(strict_types=1);

namespace App\Infrastructure\Ci\Travis;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\BuildTrigger;
use App\Domain\Ci\CiProvider;
use App\Domain\Shared\CommitSha;
use App\Domain\Shared\MonthRange;
use App\Domain\Shared\RepoFullName;
use App\Support\Saloon\PayloadHelpers;
use Carbon\CarbonImmutable;
use Generator;
use InvalidArgumentException;

class TravisProvider implements CiProvider
{
    private const TRUNK_BRANCHES = ['master', 'main'];

    public function __construct(private readonly TravisConnector $connector)
    {
    }

    /**
     * @return Generator<int, BuildSummary>
     */
    public function listBuildsInMonth(RepoFullName $repoFullName, MonthRange $month): Generator
    {
        $offset = 0;
        $limit = 25;

        while (true) {
            $response = $this->connector->send(new ListBuildsRequest($repoFullName, $offset, $limit));
            $payload = PayloadHelpers::stringKeyedArray($response->json());
            $builds = PayloadHelpers::listOfArrays($payload['builds'] ?? null);
            if ($builds === []) {
                break;
            }

            $reachedOlderThanRange = false;
            foreach ($builds as $rawBuild) {
                $build = $this->parseBuild($rawBuild);

                if ($build->startedAt->greaterThanOrEqualTo($month->end)) {
                    continue;
                }
                if ($build->startedAt->lessThan($month->start)) {
                    $reachedOlderThanRange = true;
                    continue;
                }

                yield $build;
            }

            if ($reachedOlderThanRange) {
                break;
            }
            if (count($builds) < $limit) {
                break;
            }
            $offset += $limit;
        }
    }

    public function getBuildLog(RepoFullName $repoFullName, string $externalBuildId): string
    {
        $buildResponse = $this->connector->send(new GetBuildRequest($externalBuildId));
        $buildPayload = PayloadHelpers::stringKeyedArray($buildResponse->json());
        $jobs = PayloadHelpers::listOfArrays($buildPayload['jobs'] ?? null);

        $logs = [];
        foreach ($jobs as $job) {
            $jobId = $job['id'] ?? null;
            $jobIdStr = match (true) {
                is_int($jobId) => (string)$jobId,
                is_string($jobId) && $jobId !== '' => $jobId,
                default => null,
            };
            if ($jobIdStr === null) {
                continue;
            }

            $logResponse = $this->connector->send(new GetJobLogRequest($jobIdStr));
            $logPayload = PayloadHelpers::stringKeyedArray($logResponse->json());
            $content = $logPayload['content'] ?? null;
            if (is_string($content)) {
                $logs[] = $content;
            }
        }

        return implode("\n", $logs);
    }

    /**
     * @param array<string, mixed> $raw
     */
    private function parseBuild(array $raw): BuildSummary
    {
        $repository = $raw['repository'] ?? null;
        $commit = $raw['commit'] ?? null;
        $branch = $raw['branch'] ?? null;

        if (! is_array($repository) || ! is_string($repository['slug'] ?? null)) {
            throw new InvalidArgumentException('Travis payload missing repository.slug');
        }
        if (! is_array($commit) || ! is_string($commit['sha'] ?? null)) {
            throw new InvalidArgumentException('Travis payload missing commit.sha');
        }
        if (! is_string($raw['event_type'] ?? null)) {
            throw new InvalidArgumentException('Travis payload missing event_type');
        }
        if (! is_string($raw['started_at'] ?? null)) {
            throw new InvalidArgumentException('Travis payload missing started_at');
        }

        $branchName = is_array($branch) && is_string($branch['name'] ?? null)
            ? $branch['name']
            : null;

        $status = is_string($raw['state'] ?? null)
            ? $this->resolveStatus($raw['state'])
            : throw new InvalidArgumentException('Travis payload missing state');

        $id = $raw['id'] ?? null;
        $externalId = match (true) {
            is_int($id) => (string)$id,
            is_string($id) && $id !== '' => $id,
            default => throw new InvalidArgumentException('Travis payload missing id'),
        };

        $duration = $raw['duration'] ?? null;

        $authorLogin = is_array($commit) && is_string($commit['author_name'] ?? null)
            ? $commit['author_name']
            : null;

        // Travis commit.author_name 是顯示名稱，author_email / committer_* 也可能有
        // 但統計用的 account 需要 GitHub login；此處先存 committer_email 作為 fallback，
        // 真實 GitHub login 應由呼叫端在抓完 PR 資料後透過 GitHubProvider 補齊。
        $authorEmail = is_array($commit) && is_string($commit['committer_email'] ?? null)
            ? $commit['committer_email']
            : null;

        $prNum = $raw['pull_request_number'] ?? null;

        return new BuildSummary(
            externalId: $externalId,
            repoFullName: new RepoFullName($repository['slug']),
            commitSha: new CommitSha($commit['sha']),
            authorAccount: $authorEmail ?? $authorLogin,
            prNumber: is_int($prNum) ? $prNum : null,
            status: $status,
            trigger: $this->resolveTrigger($raw['event_type'], $branchName),
            branch: $branchName,
            startedAt: CarbonImmutable::parse($raw['started_at'])->utc(),
            durationSeconds: is_int($duration) ? $duration : null,
        );
    }

    private function resolveStatus(string $state): BuildStatus
    {
        return match ($state) {
            'passed' => BuildStatus::PASSED,
            'failed', 'errored' => BuildStatus::FAILED,
            'canceled' => BuildStatus::CANCELED,
            default => BuildStatus::IN_PROGRESS,
        };
    }

    private function resolveTrigger(string $eventType, ?string $branch): BuildTrigger
    {
        return match ($eventType) {
            'pull_request' => BuildTrigger::PULL_REQUEST,
            'push' => in_array($branch, self::TRUNK_BRANCHES, true)
                ? BuildTrigger::POST_MERGE
                : BuildTrigger::PULL_REQUEST,
            'cron' => BuildTrigger::SCHEDULED,
            'api' => BuildTrigger::MANUAL,
            default => BuildTrigger::PULL_REQUEST,
        };
    }
}
