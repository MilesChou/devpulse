<?php

declare(strict_types=1);

namespace App\Infrastructure\Ci\Travis;

use DevPulse\Shared\RepoFullName;
use Saloon\Enums\Method;
use Saloon\Http\Request;

class ListBuildsRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(
        private readonly RepoFullName $repoFullName,
        private readonly int $offset = 0,
        private readonly int $limit = 25,
    ) {
    }

    public function resolveEndpoint(): string
    {
        return '/repo/' . rawurlencode((string)$this->repoFullName) . '/builds';
    }

    /**
     * @return array<string, int|string>
     */
    protected function defaultQuery(): array
    {
        return [
            'include' => 'build.commit,build.branch',
            'offset' => $this->offset,
            'limit' => $this->limit,
            // sort_by=id:desc 而非 started_at:desc：started_at 在 Travis API 上不是 indexed
            // column，大 repo 用 started_at 排序會 timeout（30s+）。id:desc 大致對應時間倒序，
            // 配合 listBuildsInMonth 的「連續 50 筆早於月份才停」邏輯正確處理 id 與時間的交錯。
            'sort_by' => 'id:desc',
        ];
    }
}
