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
     * 把 VO 流寫入 DB，依 (repo_id, external_id) 去重（同一 build 不重複插入）。
     *
     * 用 updateOrCreate（每筆 SELECT + INSERT/UPDATE 兩 query）而非 batch upsert：
     * Stage 1 量小（單月單 repo 約 100~1000 筆）可接受，且 updateOrCreate 自動
     * 套用 cast（JSON、enum、datetime），如果改用 Query Builder upsert 要 caller
     * 自己 json_encode raw_payload，trade-off 不划算。後續量大時再切 batch upsert。
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
                ['repo_id' => $repoId, 'external_id' => $attributes['external_id']],
                $attributes,
            );
            $count++;
        }

        return $count;
    }
}
