<?php

declare(strict_types=1);

namespace Tests\Domain\Vcs\Filter;

use DevPulse\Shared\RepoFullName;
use DevPulse\Shared\RepoId;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\ChangeStats;
use DevPulse\Vcs\Filter\BotFilter;
use DevPulse\Vcs\Platform;
use DevPulse\Vcs\PullRequest;
use DevPulse\Vcs\PullRequestId;
use DevPulse\Vcs\PullRequestNumber;
use DevPulse\Vcs\PullRequestStatus;
use DevPulse\Vcs\ReviewState;
use DevPulse\Vcs\ReviewSummary;
use Carbon\CarbonImmutable;
use PHPUnit\Framework\TestCase;

class BotFilterTest extends TestCase
{
    public function testIsBotPullRequestWhenAuthorInList(): void
    {
        $filter = new BotFilter(['dependabot[bot]', 'renovate[bot]']);
        $this->assertTrue($filter->isBotPullRequest($this->pr(author: 'dependabot[bot]')));
    }

    public function testIsBotPullRequestFalseWhenAuthorNotInList(): void
    {
        $filter = new BotFilter(['dependabot[bot]']);
        $this->assertFalse($filter->isBotPullRequest($this->pr(author: 'alice')));
    }

    public function testIsBotReviewWhenReviewerInList(): void
    {
        $filter = new BotFilter(['copilot-pull-request-reviewer[bot]']);
        $this->assertTrue($filter->isBotReview($this->review(reviewer: 'copilot-pull-request-reviewer[bot]')));
    }

    public function testIsBotReviewFalseWhenReviewerNotInList(): void
    {
        $filter = new BotFilter(['copilot-pull-request-reviewer[bot]']);
        $this->assertFalse($filter->isBotReview($this->review(reviewer: 'alice')));
    }

    public function testEmptyExcludeListNeverMatches(): void
    {
        $filter = new BotFilter([]);
        $this->assertFalse($filter->isExcluded('dependabot[bot]'));
    }

    private function pr(string $author): PullRequest
    {
        $createdAt = CarbonImmutable::parse('2026-04-15T10:00:00Z');

        return new PullRequest(
            id: new PullRequestId('01JTEST000000000000000000C'),
            platform: Platform::GitHub,
            repoId: new RepoId(1),
            number: new PullRequestNumber(1),
            author: new Author($author),
            status: PullRequestStatus::Open,
            changes: new ChangeStats(1, 1),
            createdAt: $createdAt,
            readyAt: $createdAt,
            closedAt: null,
        );
    }

    private function review(string $reviewer): ReviewSummary
    {
        return new ReviewSummary(
            repoFullName: new RepoFullName('your-org/your-repo'),
            pullRequestNumber: 1,
            reviewerAccount: $reviewer,
            state: ReviewState::Commented,
            submittedAt: CarbonImmutable::parse('2026-04-15T11:00:00Z'),
        );
    }
}
