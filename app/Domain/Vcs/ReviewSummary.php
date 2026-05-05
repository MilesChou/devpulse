<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use Carbon\CarbonImmutable;
use InvalidArgumentException;

final readonly class ReviewSummary
{
    public function __construct(
        public string $repoFullName,
        public int $pullRequestNumber,
        public string $reviewerAccount,
        public ReviewState $state,
        public CarbonImmutable $submittedAt,
    ) {
        if (! str_contains($repoFullName, '/')) {
            throw new InvalidArgumentException('repoFullName 必須是 owner/name 格式');
        }
        if ($pullRequestNumber < 1) {
            throw new InvalidArgumentException('pullRequestNumber 必須 >= 1');
        }
        if ($reviewerAccount === '') {
            throw new InvalidArgumentException('reviewerAccount 不能是空字串');
        }
    }

    /**
     * 從 GitHub GraphQL 的 review node 建立。
     *
     * @param array<string, mixed> $node
     */
    public static function fromGitHubGraphQL(array $node, string $repoFullName, int $pullRequestNumber): self
    {
        $state = $node['state'] ?? null;
        if (! is_string($state)) {
            throw new InvalidArgumentException('GitHub review node 缺少 state');
        }

        $submittedAt = $node['submittedAt'] ?? null;
        if (! is_string($submittedAt) || $submittedAt === '') {
            throw new InvalidArgumentException('GitHub review node 缺少 submittedAt');
        }

        $author = $node['author'] ?? null;
        if (! is_array($author) || ! is_string($author['login'] ?? null)) {
            throw new InvalidArgumentException('GitHub review node 缺少 author.login');
        }

        return new self(
            repoFullName: $repoFullName,
            pullRequestNumber: $pullRequestNumber,
            reviewerAccount: $author['login'],
            state: ReviewState::fromGitHubGraphQL($state),
            submittedAt: CarbonImmutable::parse($submittedAt)->utc(),
        );
    }
}
