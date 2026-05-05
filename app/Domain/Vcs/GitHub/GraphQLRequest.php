<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

use Saloon\Contracts\Body\HasBody;
use Saloon\Enums\Method;
use Saloon\Http\Request;
use Saloon\Traits\Body\HasJsonBody;

abstract class GraphQLRequest extends Request implements HasBody
{
    use HasJsonBody;

    protected Method $method = Method::POST;

    public function resolveEndpoint(): string
    {
        return '/graphql';
    }

    abstract protected function graphqlQuery(): string;

    /**
     * @return array<string, mixed>
     */
    abstract protected function graphqlVariables(): array;

    /**
     * @return array<string, mixed>
     */
    protected function defaultBody(): array
    {
        return [
            'query' => $this->graphqlQuery(),
            'variables' => $this->graphqlVariables(),
        ];
    }
}
