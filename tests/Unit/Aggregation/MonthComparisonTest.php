<?php

declare(strict_types=1);

namespace Tests\Unit\Aggregation;

use App\Aggregation\Dto\ComparisonDirection;
use App\Aggregation\Dto\MonthComparison;
use PHPUnit\Framework\TestCase;

class MonthComparisonTest extends TestCase
{
    public function testDirectionUpWhenCurrentHigher(): void
    {
        $cmp = new MonthComparison(current: 0.05, previous: 0.03);
        $this->assertSame(ComparisonDirection::Up, $cmp->direction);
        $this->assertEqualsWithDelta(0.02, $cmp->delta, 1e-9);
    }

    public function testDirectionDownWhenCurrentLower(): void
    {
        $cmp = new MonthComparison(current: 0.03, previous: 0.05);
        $this->assertSame(ComparisonDirection::Down, $cmp->direction);
        $this->assertEqualsWithDelta(0.02, $cmp->delta, 1e-9);
    }

    public function testDirectionNeutralWhenEqual(): void
    {
        $cmp = new MonthComparison(current: 0.05, previous: 0.05);
        $this->assertSame(ComparisonDirection::Neutral, $cmp->direction);
        $this->assertSame(0.0, $cmp->delta);
    }

    public function testDirectionNeutralWhenNoPrevious(): void
    {
        $cmp = new MonthComparison(current: 0.05, previous: null);
        $this->assertSame(ComparisonDirection::Neutral, $cmp->direction);
        $this->assertSame(0.0, $cmp->delta);
    }

    public function testFormatUpWithPercent(): void
    {
        $cmp = new MonthComparison(current: 0.05, previous: 0.03);
        $formatted = $cmp->format(decimals: 2, suffix: '%');
        $this->assertSame('↑ +0.02%', $formatted);
    }

    public function testFormatDownWithPercent(): void
    {
        $cmp = new MonthComparison(current: 0.03, previous: 0.05);
        $formatted = $cmp->format(decimals: 2, suffix: '%');
        $this->assertSame('↓ -0.02%', $formatted);
    }

    public function testFormatNeutralReturnsArrowOnly(): void
    {
        $cmp = new MonthComparison(current: 0.05, previous: null);
        $this->assertSame('→', $cmp->format());
    }
}
