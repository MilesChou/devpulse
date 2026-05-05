<?php

declare(strict_types=1);

namespace App\Infrastructure\Ci\Travis;

use Saloon\Enums\Method;
use Saloon\Http\Request;

class GetBuildRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(private readonly string $buildId)
    {
    }

    public function resolveEndpoint(): string
    {
        return '/build/' . rawurlencode($this->buildId);
    }

    /**
     * @return array<string, string>
     */
    protected function defaultQuery(): array
    {
        return [
            'include' => 'build.jobs',
        ];
    }
}
