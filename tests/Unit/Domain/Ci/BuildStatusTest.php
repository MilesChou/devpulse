<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Ci;

use App\Domain\Ci\BuildStatus;
use PHPUnit\Framework\TestCase;

class BuildStatusTest extends TestCase
{
    public function testIsFailureOnlyForFailed(): void
    {
        $this->assertTrue(BuildStatus::FAILED->isFailure());
        $this->assertFalse(BuildStatus::PASSED->isFailure());
        $this->assertFalse(BuildStatus::CANCELED->isFailure());
        $this->assertFalse(BuildStatus::IN_PROGRESS->isFailure());
    }

    public function testIsCompleteFalseForInProgress(): void
    {
        $this->assertFalse(BuildStatus::IN_PROGRESS->isComplete());
        $this->assertTrue(BuildStatus::PASSED->isComplete());
        $this->assertTrue(BuildStatus::FAILED->isComplete());
        $this->assertTrue(BuildStatus::CANCELED->isComplete());
    }
}
