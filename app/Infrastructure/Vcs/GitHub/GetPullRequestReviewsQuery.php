<?php

declare(strict_types=1);

namespace App\Infrastructure\Vcs\GitHub;

use App\Domain\Shared\RepoFullName;

class GetPullRequestReviewsQuery extends GraphQLRequest
{
    public function __construct(
        private readonly RepoFullName $repoFullName,
        private readonly int $pullNumber,
    ) {
    }

    protected function graphqlQuery(): string
    {
        return <<<'GRAPHQL'
            query($owner: String!, $name: String!, $number: Int!) {
              repository(owner: $owner, name: $name) {
                pullRequest(number: $number) {
                  reviews(first: 100) {
                    nodes {
                      state
                      submittedAt
                      author { login }
                    }
                  }
                }
              }
            }
            GRAPHQL;
    }

    /**
     * @return array<string, mixed>
     */
    protected function graphqlVariables(): array
    {
        return [
            'owner' => $this->repoFullName->owner,
            'name' => $this->repoFullName->name,
            'number' => $this->pullNumber,
        ];
    }
}
