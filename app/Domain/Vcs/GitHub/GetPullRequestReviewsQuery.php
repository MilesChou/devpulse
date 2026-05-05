<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

class GetPullRequestReviewsQuery extends GraphQLRequest
{
    public function __construct(
        private readonly string $repoFullName,
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
        [$owner, $name] = explode('/', $this->repoFullName, 2);

        return [
            'owner' => $owner,
            'name' => $name,
            'number' => $this->pullNumber,
        ];
    }
}
