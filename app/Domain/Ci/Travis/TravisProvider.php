<?php

declare(strict_types=1);

namespace App\Domain\Ci\Travis;

use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\CiProvider;
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
            $payload = $this->stringKeyedArray($response->json());
            $builds = $this->listOfArrays($payload['builds'] ?? null);
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
        $buildPayload = $this->stringKeyedArray($buildResponse->json());
        $jobs = $this->listOfArrays($buildPayload['jobs'] ?? null);

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
            $logPayload = $this->stringKeyedArray($logResponse->json());
            $content = $logPayload['content'] ?? null;
            if (is_string($content)) {
                $logs[] = $content;
            }
        }

        return implode("\n", $logs);
    }

    /**
     * @param mixed $value
     * @return array<string, mixed>
     */
    private function stringKeyedArray($value): array
    {
        if (! is_array($value)) {
            return [];
        }
        $result = [];
        foreach ($value as $key => $item) {
            if (is_string($key)) {
                $result[$key] = $item;
            }
        }

        return $result;
    }

    /**
     * @param mixed $value
     * @return list<array<string, mixed>>
     */
    private function listOfArrays($value): array
    {
        if (! is_array($value)) {
            return [];
        }
        $result = [];
        foreach ($value as $item) {
            if (is_array($item)) {
                $result[] = $this->stringKeyedArray($item);
            }
        }

        return $result;
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
