<?php

declare(strict_types=1);

namespace App\Domain\Ci\Travis;

use Saloon\Enums\Method;
use Saloon\Http\Request;

class ListBuildsRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(
        private readonly string $repoFullName,
        private readonly int $offset = 0,
        private readonly int $limit = 25,
    ) {
    }

    public function resolveEndpoint(): string
    {
        return '/repo/' . rawurlencode($this->repoFullName) . '/builds';
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
            'sort_by' => 'started_at:desc',
        ];
    }
}
