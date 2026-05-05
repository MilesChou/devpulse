<?php

declare(strict_types=1);

namespace App\Domain\Ci\Travis;

use Saloon\Http\Auth\TokenAuthenticator;
use Saloon\Http\Connector;
use Saloon\Traits\Plugins\AcceptsJson;

class TravisConnector extends Connector
{
    use AcceptsJson;

    public function __construct(private readonly string $token)
    {
    }

    public function resolveBaseUrl(): string
    {
        return 'https://api.travis-ci.com';
    }

    /**
     * @return array<string, string>
     */
    protected function defaultHeaders(): array
    {
        return [
            'Travis-API-Version' => '3',
        ];
    }

    protected function defaultAuth(): TokenAuthenticator
    {
        return new TokenAuthenticator($this->token, prefix: 'token');
    }
}
