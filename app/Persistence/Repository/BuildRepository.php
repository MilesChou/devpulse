<?php

declare(strict_types=1);

namespace App\Persistence\Repository;

use App\Domain\Ci\BuildSummary;
use App\Models\Build;
use App\Persistence\Mapper\BuildMapper;

final class BuildRepository
{
    public function __construct(private readonly BuildMapper $mapper)
    {
    }

    /**
     * 把 VO 流寫入 DB，依 (provider, external_id) 去重（同一 build 不重複插入）。
     *
     * 用 firstOrCreate 而非 batch upsert，因為 raw_payload 是 JSON 欄位，
     * batch upsert 在 SQLite/PostgreSQL 對 JSON 行為有差異。Stage 1 量小，
     * 一筆一筆寫可接受。
     *
     * @param iterable<BuildSummary> $builds
     * @param array<string, array<string, mixed>> $rawPayloads externalId => raw payload
     */
    public function upsertMany(int $repoId, iterable $builds, array $rawPayloads = []): int
    {
        $count = 0;
        foreach ($builds as $vo) {
            $payload = $rawPayloads[$vo->externalId] ?? [];
            $attributes = $this->mapper->toAttributes($vo, $repoId, $payload);

            Build::query()->updateOrCreate(
                ['provider' => $attributes['provider'], 'external_id' => $attributes['external_id']],
                $attributes,
            );
            $count++;
        }

        return $count;
    }
}
