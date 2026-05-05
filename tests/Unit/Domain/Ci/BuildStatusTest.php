<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Ci;

use App\Domain\Ci\BuildStatus;
use PHPUnit\Framework\TestCase;

class BuildStatusTest extends TestCase
{
    public function testIsFailureOnlyForFailedAndErrored(): void
    {
        $this->assertTrue(BuildStatus::Failed->isFailure());
        $this->assertTrue(BuildStatus::Errored->isFailure());
        $this->assertFalse(BuildStatus::Passed->isFailure());
        $this->assertFalse(BuildStatus::Canceled->isFailure());
        $this->assertFalse(BuildStatus::Created->isFailure());
        $this->assertFalse(BuildStatus::Started->isFailure());
    }

    public function testIsCompleteFalseForCreatedAndStarted(): void
    {
        $this->assertFalse(BuildStatus::Created->isComplete());
        $this->assertFalse(BuildStatus::Started->isComplete());
        $this->assertTrue(BuildStatus::Passed->isComplete());
        $this->assertTrue(BuildStatus::Failed->isComplete());
        $this->assertTrue(BuildStatus::Errored->isComplete());
        $this->assertTrue(BuildStatus::Canceled->isComplete());
    }
}
