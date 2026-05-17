<?php

declare(strict_types=1);

namespace DevPulse\Vcs;

use DateTimeImmutable;
use DevPulse\Shared\RepoFullName;
use DevPulse\Shared\UtcTimestamp;
use InvalidArgumentException;

final readonly class ReviewSummary
{
    public function __construct(
        public RepoFullName $repoFullName,
        public int $pullRequestNumber,
        public string $reviewerAccount,
        public ReviewState $state,
        public DateTimeImmutable $submittedAt,
    ) {
        if ($pullRequestNumber < 1) {
            throw new InvalidArgumentException('pullRequestNumber must be >= 1');
        }
        if ($reviewerAccount === '') {
            throw new InvalidArgumentException('reviewerAccount must not be empty');
        }
    }

    /**
     * 從 GitHub GraphQL 的 review node 建立。
     *
     * @param array<string, mixed> $node
     */
    public static function fromGitHubGraphQL(array $node, RepoFullName $repoFullName, int $pullRequestNumber): self
    {
        $state = $node['state'] ?? null;
        if (! is_string($state)) {
            throw new InvalidArgumentException('GitHub review node missing state');
        }

        $author = $node['author'] ?? null;
        if (! is_array($author) || ! is_string($author['login'] ?? null)) {
            throw new InvalidArgumentException('GitHub review node missing author.login');
        }

        return new self(
            repoFullName: $repoFullName,
            pullRequestNumber: $pullRequestNumber,
            reviewerAccount: $author['login'],
            state: ReviewState::fromGitHubGraphQL($state),
            submittedAt: UtcTimestamp::required($node, 'submittedAt', 'GitHub review node missing submittedAt'),
        );
    }
}
