<?php

declare(strict_types=1);

namespace App\Infrastructure\Ci\Travis;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\Build;
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
     * Travis API 的 sort_by=id:desc 不嚴格等於時間倒序：cancelled / re-run / pull_request
     * vs push 的 build id 與時間軸會交錯。所以「遇到第一筆早於月份就 break」是錯的，
     * 必須累積「連續 N 筆早於月份」才能安全停（沿用 Python prototype 的 50 筆閾值）。
     *
     * @return Generator<int, Build>
     */
    public function listBuildsInMonth(RepoFullName $repoFullName, MonthRange $month): Generator
    {
        $offset = 0;
        $limit = 100;
        $consecutiveBelow = 0;

        while (true) {
            $response = $this->connector->send(new ListBuildsRequest($repoFullName, $offset, $limit));
            $payload = PayloadHelpers::stringKeyedArray($response->json());
            $builds = PayloadHelpers::listOfArrays($payload['builds'] ?? null);
            if ($builds === []) {
                break;
            }

            $stop = false;
            foreach ($builds as $rawBuild) {
                try {
                    $build = $this->parseBuild($rawBuild);
                } catch (InvalidArgumentException) {
                    // 跳過無法解析的 build（例如 state=canceled 且 started_at / finished_at 都 null）
                    // 這類 build 沒有時間軸資訊、無法歸入任何月份，硬要 throw 會中斷整批
                    continue;
                }

                if ($build->startedAt->greaterThanOrEqualTo($month->end)) {
                    $consecutiveBelow = 0;
                    continue;
                }
                if ($build->startedAt->lessThan($month->start)) {
                    $consecutiveBelow++;
                    if ($consecutiveBelow >= 50) {
                        $stop = true;
                        break;
                    }
                    continue;
                }

                $consecutiveBelow = 0;
                yield $build;
            }

            if ($stop) {
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
    private function parseBuild(array $raw): Build
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
        // Travis 對 state=canceled / errored 的 build，started_at 可能為 null
        // 退而求其次用 finished_at；都沒有才 throw（這種 build 確實沒時間軸可放）
        $startedAtRaw = $raw['started_at'] ?? null;
        if (! is_string($startedAtRaw)) {
            $startedAtRaw = $raw['finished_at'] ?? null;
        }
        if (! is_string($startedAtRaw)) {
            throw new InvalidArgumentException('Travis payload missing started_at and finished_at');
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

        // Travis commit payload 只有 author.name（顯示名）+ avatar_url，無 GitHub login 也無 email。
        // 真正的 GitHub login 必須由 caller（FetchOrchestrator）抓完 builds 後，
        // 用 commit sha 透過 GitHubProvider::getCommitAuthorAccount(s) 補齊。
        // 這裡留 null，避免「假的 author_account 污染下游 query」。
        $prNum = $raw['pull_request_number'] ?? null;

        return new Build(
            externalId: $externalId,
            repoFullName: new RepoFullName($repository['slug']),
            commitSha: new CommitSha($commit['sha']),
            authorAccount: null,
            prNumber: is_int($prNum) ? $prNum : null,
            status: $status,
            trigger: $this->resolveTrigger($raw['event_type'], $branchName),
            branch: $branchName,
            startedAt: CarbonImmutable::parse($startedAtRaw)->utc(),
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
