<?php

declare(strict_types=1);

namespace Unit\Domain\Vcs\Filter;

use App\Domain\Shared\RepoFullName;
use App\Domain\Shared\RepoId;
use App\Domain\Vcs\Author;
use App\Domain\Vcs\Filter\BotFilter;
use App\Domain\Vcs\PullRequest;
use App\Domain\Vcs\PullRequestNumber;
use App\Domain\Vcs\PullRequestStatus;
use App\Domain\Vcs\ReviewState;
use App\Domain\Vcs\ReviewSummary;
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
        $createdAt = CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');

        return new PullRequest(
            repoId: new RepoId(1),
            number: new PullRequestNumber(1),
            author: new Author($author),
            status: PullRequestStatus::Open,
            additions: 1,
            deletions: 1,
            createdAt: $createdAt,
            readyAt: $createdAt,
            mergedAt: null,
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
            submittedAt: CarbonImmutable::create(2026, 4, 15, 11, 0, 0, 'UTC'),
        );
    }
}
