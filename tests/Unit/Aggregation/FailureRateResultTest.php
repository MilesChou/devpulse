<?php

declare(strict_types=1);

namespace Tests\Unit\Aggregation;

use App\Aggregation\Dto\FailureRateResult;
use App\Domain\Shared\RepoFullName;
use PHPUnit\Framework\TestCase;

class FailureRateResultTest extends TestCase
{
    public function testRateCalculatedCorrectly(): void
    {
        $result = FailureRateResult::from(new RepoFullName('org/repo'), 'alice', total: 10, failures: 2);
        $this->assertEqualsWithDelta(0.2, $result->rate, 1e-9);
    }

    public function testRateIsZeroWhenNoBuilds(): void
    {
        $result = FailureRateResult::from(new RepoFullName('org/repo'), 'alice', total: 0, failures: 0);
        $this->assertSame(0.0, $result->rate);
    }

    public function testCarriesFields(): void
    {
        $result = FailureRateResult::from(new RepoFullName('org/repo'), 'bob', total: 5, failures: 1);
        $this->assertSame('org/repo', (string)$result->repoFullName);
        $this->assertSame('bob', $result->authorAccount);
        $this->assertSame(5, $result->total);
        $this->assertSame(1, $result->failures);
    }
}
