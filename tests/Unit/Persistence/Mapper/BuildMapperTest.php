<?php

declare(strict_types=1);

namespace Tests\Unit\Persistence\Mapper;

use DevPulse\Ci\Build;
use DevPulse\Ci\BuildStatus;
use DevPulse\Ci\BuildTrigger;
use DevPulse\Shared\CommitSha;
use DevPulse\Shared\RepoFullName;
use App\Persistence\Mapper\BuildMapper;
use Carbon\CarbonImmutable;
use PHPUnit\Framework\TestCase;

class BuildMapperTest extends TestCase
{
    public function testToAttributesIncludesDerivedDimensions(): void
    {
        $vo = new Build(
            externalId: '12345',
            repoFullName: new RepoFullName('your-org/your-repo'),
            commitSha: new CommitSha('abcdef0123'),
            authorAccount: 'alice',
            prNumber: null,
            status: BuildStatus::FAILED,
            trigger: BuildTrigger::POST_MERGE,
            branch: 'master',
            startedAt: CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC'),
            durationSeconds: 120,
        );

        $attributes = new BuildMapper()->toAttributes($vo, repoId: '01JTESTREP00000000000000A7', rawPayload: ['raw' => 'data']);

        $this->assertSame('01JTESTREP00000000000000A7', $attributes['repo_id']);
        $this->assertSame('12345', $attributes['external_id']);
        $this->assertSame(BuildStatus::FAILED->name, $attributes['status']);
        $this->assertTrue($attributes['is_post_merge']);
        $this->assertFalse($attributes['is_pull_request']);
        $this->assertFalse($attributes['is_deploy_event']);
        $this->assertTrue($attributes['is_failure']);
        $this->assertSame(['raw' => 'data'], $attributes['raw_payload']);
    }
}
