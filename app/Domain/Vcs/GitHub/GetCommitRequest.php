<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

use Saloon\Enums\Method;
use Saloon\Http\Request;

class GetCommitRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(
        private readonly string $repoFullName,
        private readonly string $sha,
    ) {
    }

    public function resolveEndpoint(): string
    {
        return '/repos/' . $this->repoFullName . '/commits/' . rawurlencode($this->sha);
    }
}
