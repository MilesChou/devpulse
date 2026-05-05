<?php

declare(strict_types=1);

namespace Tests\Feature\Persistence\Repository;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\BuildTrigger;
use App\Domain\Shared\CommitSha;
use App\Domain\Shared\RepoFullName;
use App\Models\Build;
use App\Models\Repo;
use App\Persistence\Mapper\BuildMapper;
use App\Persistence\Repository\BuildRepository;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class BuildRepositoryTest extends TestCase
{
    use RefreshDatabase;

    public function testInsertsNewBuilds(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $repository = new BuildRepository(new BuildMapper());

        $count = $repository->upsertMany($repo->id, [
            $this->build('1', '2026-04-15T10:00:00Z'),
            $this->build('2', '2026-04-16T10:00:00Z'),
        ]);

        $this->assertSame(2, $count);
        $this->assertSame(2, Build::query()->count());
    }

    public function testIsIdempotentOnSameExternalId(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $repository = new BuildRepository(new BuildMapper());

        $repository->upsertMany($repo->id, [$this->build('1', '2026-04-15T10:00:00Z')]);
        $repository->upsertMany($repo->id, [$this->build('1', '2026-04-15T10:00:00Z')]);

        $this->assertSame(1, Build::query()->count());
    }

    public function testUpdatesExistingBuildOnRefetch(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $repository = new BuildRepository(new BuildMapper());

        $repository->upsertMany($repo->id, [$this->build('1', '2026-04-15T10:00:00Z', BuildStatus::IN_PROGRESS)]);
        $repository->upsertMany($repo->id, [$this->build('1', '2026-04-15T10:00:00Z', BuildStatus::PASSED)]);

        $build = Build::query()->where('external_id', '1')->first();
        $this->assertNotNull($build);
        $this->assertSame(BuildStatus::PASSED->name, $build->status);
    }

    public function testStoresRawPayload(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $repository = new BuildRepository(new BuildMapper());

        $repository->upsertMany(
            $repo->id,
            [$this->build('1', '2026-04-15T10:00:00Z')],
            ['1' => ['some' => 'payload']],
        );

        $build = Build::query()->where('external_id', '1')->first();
        $this->assertSame(['some' => 'payload'], $build->raw_payload);
    }

    private function build(string $externalId, string $startedAt, BuildStatus $status = BuildStatus::PASSED): BuildSummary
    {
        return new BuildSummary(
            externalId: $externalId,
            repoFullName: new RepoFullName('your-org/your-repo'),
            commitSha: new CommitSha('abcdef0'),
            authorAccount: 'alice',
            prNumber: null,
            status: $status,
            trigger: BuildTrigger::PULL_REQUEST,
            branch: 'feature/foo',
            startedAt: CarbonImmutable::parse($startedAt)->utc(),
            durationSeconds: 120,
        );
    }
}
