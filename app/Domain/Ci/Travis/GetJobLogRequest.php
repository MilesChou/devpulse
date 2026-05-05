<?php

declare(strict_types=1);

namespace App\Domain\Ci\Travis;

use Saloon\Enums\Method;
use Saloon\Http\Request;

class GetJobLogRequest extends Request
{
    protected Method $method = Method::GET;

    public function __construct(private readonly string $jobId)
    {
    }

    public function resolveEndpoint(): string
    {
        return '/job/' . rawurlencode($this->jobId) . '/log';
    }
}
