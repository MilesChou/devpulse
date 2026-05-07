<?php

declare(strict_types=1);

namespace Tests\Domain\Vcs;

use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\ReviewState;
use DevPulse\Vcs\ReviewSummary;
use Carbon\CarbonImmutable;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class ReviewSummaryTest extends TestCase
{
    public function testBuildsValidInstance(): void
    {
        $review = $this->build();
        $this->assertSame('your-org/your-repo', (string)$review->repoFullName);
        $this->assertSame(42, $review->pullRequestNumber);
        $this->assertSame(ReviewState::Approved, $review->state);
    }

    public function testThrowsWhenPullRequestNumberZero(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('pullRequestNumber');
        $this->build(pullRequestNumber: 0);
    }

    public function testThrowsWhenReviewerAccountIsEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('reviewerAccount');
        $this->build(reviewerAccount: '');
    }

    private function build(
        int $pullRequestNumber = 42,
        string $reviewerAccount = 'reviewer',
        ReviewState $state = ReviewState::Approved,
        ?CarbonImmutable $submittedAt = null,
    ): ReviewSummary {
        return new ReviewSummary(
            repoFullName: new RepoFullName('your-org/your-repo'),
            pullRequestNumber: $pullRequestNumber,
            reviewerAccount: $reviewerAccount,
            state: $state,
            submittedAt: $submittedAt ?? CarbonImmutable::parse('2026-04-15T11:00:00Z'),
        );
    }
}
