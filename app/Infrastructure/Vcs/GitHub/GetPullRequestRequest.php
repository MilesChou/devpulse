<?php

declare(strict_types=1);

namespace App\Infrastructure\Vcs\GitHub;

use Saloon\Enums\Method;
use Saloon\Http\Request;

class GetPullRequestRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(
        private readonly string $repoFullName,
        private readonly int $pullNumber,
    ) {
    }

    public function resolveEndpoint(): string
    {
        return '/repos/' . $this->repoFullName . '/pulls/' . $this->pullNumber;
    }
}
