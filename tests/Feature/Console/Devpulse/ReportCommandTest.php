<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use DevPulse\Ci\BuildStatus;
use DevPulse\Ci\BuildTrigger;
use DevPulse\Vcs\PullRequestStatus;
use App\Models\Build;
use App\Models\Group;
use App\Models\PullRequest;
use App\Models\Repo;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\File;
use Tests\Support\CreatesRepoModel;
use Tests\TestCase;

class ReportCommandTest extends TestCase
{
    use CreatesRepoModel;
    use RefreshDatabase;

    public function testGeneratesMarkdownReportWithAllSections(): void
    {
        $this->seedSampleData();

        $this->artisan('devpulse:report', [
            'month' => '2026-04',
            '--group' => 'team-a',
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('# devpulse 月報：team-a / 2026-04')
            ->expectsOutputToContain('## CI 失敗率')
            ->expectsOutputToContain('## PR Review Latency')
            ->expectsOutputToContain('## 每日 Build 時間趨勢')
            ->expectsOutputToContain('## 失敗 Build 清單');
    }

    public function testFailsWhenGroupOptionMissing(): void
    {
        $this->artisan('devpulse:report', ['month' => '2026-04'])
            ->assertFailed();
    }

    public function testFailsWhenGroupDoesNotExist(): void
    {
        $this->artisan('devpulse:report', [
            'month' => '2026-04',
            '--group' => 'nonexistent',
        ])->assertFailed();
    }

    public function testFailsWhenMonthFormatInvalid(): void
    {
        Group::create(['slug' => 'team-a']);
        $this->artisan('devpulse:report', [
            'month' => 'invalid-month',
            '--group' => 'team-a',
        ])->assertFailed();
    }

    public function testWritesReportToFileWhenOutputProvided(): void
    {
        $this->seedSampleData();

        $tmp = sys_get_temp_dir() . '/devpulse-report-' . uniqid() . '.md';
        try {
            $this->artisan('devpulse:report', [
                'month' => '2026-04',
                '--group' => 'team-a',
                '--output' => $tmp,
            ])->assertSuccessful();

            $this->assertFileExists($tmp);
            $content = (string)file_get_contents($tmp);
            $this->assertStringContainsString('# devpulse 月報：team-a / 2026-04', $content);
            $this->assertStringContainsString('## CI 失敗率', $content);
        } finally {
            if (File::exists($tmp)) {
                File::delete($tmp);
            }
        }
    }

    public function testEmptyDataStillProducesValidMarkdown(): void
    {
        Group::create(['slug' => 'empty-team']);

        $this->artisan('devpulse:report', [
            'month' => '2026-04',
            '--group' => 'empty-team',
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('（本月無資料）')
            ->expectsOutputToContain('（本月無失敗 build）');
    }

    private function seedSampleData(): void
    {
        $group = Group::create(['slug' => 'team-a', 'description' => 'Team A']);
        $repo = $this->makeRepo('org/repo-a');
        $group->repos()->attach($repo->id);

        // 3 個 PR build：alice 2 次（1 失敗）、bob 1 次（成功）
        $this->insertBuild($repo->id, 'alice', '2026-04-10T10:00:00Z', BuildStatus::PASSED, prNumber: 101);
        $this->insertBuild($repo->id, 'alice', '2026-04-11T10:00:00Z', BuildStatus::FAILED, prNumber: 101);
        $this->insertBuild($repo->id, 'bob', '2026-04-12T10:00:00Z', BuildStatus::PASSED, prNumber: 102);

        // 1 個 PR 帶 review 資料
        PullRequest::create([
            'repo_id' => $repo->id,
            'number' => 101,
            'author_account' => 'alice',
            'status' => PullRequestStatus::Merged->value,
            'additions' => 30,
            'deletions' => 10,
            'total_changed_lines' => 40,
            'size_bucket' => 'XS',
            'is_draft' => false,
            'pr_created_at' => CarbonImmutable::parse('2026-04-09T10:00:00Z'),
            'ready_at' => CarbonImmutable::parse('2026-04-09T10:00:00Z'),
            'first_review_at' => CarbonImmutable::parse('2026-04-09T12:30:00Z'),
            'merged_at' => CarbonImmutable::parse('2026-04-10T15:00:00Z'),
            'closed_at' => null,
            'raw_payload' => [],
        ]);
    }

    private function insertBuild(
        string $repoId,
        string $author,
        string $startedAt,
        BuildStatus $status,
        ?int $prNumber = null,
    ): void {
        Build::create([
            'repo_id' => $repoId,
            'external_id' => uniqid(),
            'commit_sha' => str_repeat('a', 40),
            'author_account' => $author,
            'pr_number' => $prNumber,
            'status' => $status->name,
            'trigger' => BuildTrigger::PULL_REQUEST->name,
            'branch' => 'feature/test',
            'is_post_merge' => false,
            'is_pull_request' => true,
            'is_deploy_event' => false,
            'is_failure' => $status->isFailure(),
            'started_at' => CarbonImmutable::parse($startedAt),
            'duration_seconds' => 120,
            'raw_payload' => [],
        ]);
    }
}
