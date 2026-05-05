<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

use Saloon\Enums\Method;
use Saloon\Http\Request;

class ListPullRequestsRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(
        private readonly string $repoFullName,
        private readonly int $page = 1,
        private readonly int $perPage = 100,
    ) {
    }

    public function resolveEndpoint(): string
    {
        return '/repos/' . $this->repoFullName . '/pulls';
    }

    /**
     * @return array<string, int|string>
     */
    protected function defaultQuery(): array
    {
        return [
            'state' => 'all',
            'sort' => 'created',
            'direction' => 'desc',
            'per_page' => $this->perPage,
            'page' => $this->page,
        ];
    }
}
