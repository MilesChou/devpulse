<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Shared;

use App\Domain\Shared\RepoFullName;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class RepoFullNameTest extends TestCase
{
    public function testParsesOwnerAndName(): void
    {
        $repo = new RepoFullName('your-org/your-repo');
        $this->assertSame('your-org', $repo->owner);
        $this->assertSame('your-repo', $repo->name);
    }

    public function testToStringReturnsFullName(): void
    {
        $this->assertSame('your-org/your-repo', (string)new RepoFullName('your-org/your-repo'));
    }

    public function testThrowsWhenNoSlash(): void
    {
        $this->expectException(InvalidArgumentException::class);
        new RepoFullName('invalid');
    }

    public function testThrowsWhenOwnerEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        new RepoFullName('/repo');
    }

    public function testThrowsWhenNameEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        new RepoFullName('owner/');
    }

    public function testAllowsSlashInName(): void
    {
        $repo = new RepoFullName('org/repo/nested');
        $this->assertSame('org', $repo->owner);
        $this->assertSame('repo/nested', $repo->name);
    }
}
