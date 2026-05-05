<?php

declare(strict_types=1);

namespace App\Domain\Ci\Travis;

use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\CiProvider;
use App\Support\Saloon\PayloadHelpers;
use Carbon\CarbonImmutable;
use Generator;
use InvalidArgumentException;

class TravisProvider implements CiProvider
{
    public function __construct(private readonly TravisConnector $connector)
    {
    }

    /**
     * @return Generator<int, BuildSummary>
     */
    public function listBuildsInMonth(string $repoFullName, string $month): Generator
    {
        [$start, $end] = $this->monthRange($month);
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
                $build = BuildSummary::fromTravisRaw($rawBuild);

                if ($build->startedAt->greaterThanOrEqualTo($end)) {
                    continue;
                }
                if ($build->startedAt->lessThan($start)) {
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

    public function getBuildLog(string $repoFullName, string $externalBuildId): string
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
     * @return array{0: CarbonImmutable, 1: CarbonImmutable}
     */
    private function monthRange(string $month): array
    {
        $parsed = CarbonImmutable::createFromFormat('Y-m', $month, 'UTC');
        if (! $parsed instanceof CarbonImmutable) {
            throw new InvalidArgumentException("month 格式錯誤：必須是 YYYY-MM（收到 `{$month}`）");
        }
        $start = $parsed->startOfMonth();
        $end = $start->addMonth();

        return [$start, $end];
    }
}
