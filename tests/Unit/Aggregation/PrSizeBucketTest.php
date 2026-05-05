<?php

declare(strict_types=1);

namespace Tests\Unit\Aggregation;

use App\Aggregation\PrSizeBucket;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class PrSizeBucketTest extends TestCase
{
    private PrSizeBucket $bucket;

    protected function setUp(): void
    {
        parent::setUp();
        $this->bucket = new PrSizeBucket([
            'XS' => 50,
            'S' => 200,
            'M' => 500,
            'L' => 1000,
            'XL' => null,
        ]);
    }

    public function testClassifiesSmallPrAsXs(): void
    {
        $this->assertSame('XS', $this->bucket->classify(49));
        $this->assertSame('XS', $this->bucket->classify(0));
    }

    public function testBoundaryIsExclusive(): void
    {
        $this->assertSame('XS', $this->bucket->classify(49));
        $this->assertSame('S', $this->bucket->classify(50));
    }

    public function testClassifiesMediumBuckets(): void
    {
        $this->assertSame('S', $this->bucket->classify(199));
        $this->assertSame('M', $this->bucket->classify(200));
        $this->assertSame('M', $this->bucket->classify(499));
        $this->assertSame('L', $this->bucket->classify(500));
        $this->assertSame('L', $this->bucket->classify(999));
    }

    public function testClassifiesLargePrAsXl(): void
    {
        $this->assertSame('XL', $this->bucket->classify(1000));
        $this->assertSame('XL', $this->bucket->classify(9999));
    }

    public function testThrowsWhenEmptyConfig(): void
    {
        $this->expectException(InvalidArgumentException::class);
        new PrSizeBucket([]);
    }
}
