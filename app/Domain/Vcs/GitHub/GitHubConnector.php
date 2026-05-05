<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

use Saloon\Exceptions\Request\FatalRequestException;
use Saloon\Exceptions\Request\RequestException;
use Saloon\Http\Auth\TokenAuthenticator;
use Saloon\Http\Connector;
use Saloon\Http\Request;
use Saloon\Traits\Plugins\AcceptsJson;

class GitHubConnector extends Connector
{
    use AcceptsJson;

    public ?int $tries = 3;
    public ?int $retryInterval = 1000;
    public ?bool $useExponentialBackoff = true;

    public function __construct(private readonly string $token)
    {
    }

    /**
     * 只對 rate limit (429) 與 server errors (5xx) 與網路層失敗重試。
     * 4xx auth / not found / secondary rate limit 等不重試（重試也不會好）。
     */
    public function handleRetry(FatalRequestException|RequestException $exception, Request $request): bool
    {
        if ($exception instanceof FatalRequestException) {
            return true;
        }

        $status = $exception->getStatus();

        return $status === 429 || $status >= 500;
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
