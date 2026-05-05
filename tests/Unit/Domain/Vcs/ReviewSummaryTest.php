<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Vcs;

use App\Domain\Shared\RepoFullName;
use App\Domain\Vcs\ReviewState;
use App\Domain\Vcs\ReviewSummary;
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
            submittedAt: $submittedAt ?? CarbonImmutable::create(2026, 4, 15, 11, 0, 0, 'UTC'),
        );
    }
}
