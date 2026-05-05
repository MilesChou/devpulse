<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

use Saloon\Http\Auth\TokenAuthenticator;
use Saloon\Http\Connector;
use Saloon\Traits\Plugins\AcceptsJson;

class GitHubConnector extends Connector
{
    use AcceptsJson;

    public function __construct(private readonly string $token)
    {
    }

    public function resolveBaseUrl(): string
    {
        return 'https://api.github.com';
    }

    /**
     * @return array<string, string>
     */
    protected function defaultHeaders(): array
    {
        return [
            'Accept' => 'application/vnd.github+json',
            'X-GitHub-Api-Version' => '2022-11-28',
        ];
    }

    protected function defaultAuth(): TokenAuthenticator
    {
        return new TokenAuthenticator($this->token, prefix: 'Bearer');
    }
}
