<?php

declare(strict_types=1);

namespace App\Console\Commands\Devpulse;

use App\Aggregation\BuildFailureRateQuery;
use App\Aggregation\DailyBuildDurationQuery;
use App\Aggregation\FailedBuildListQuery;
use App\Aggregation\ReviewLatencyQuery;
use DevPulse\Shared\MonthRange;
use App\Models\Group;
use App\Reporting\Renderer\DailyBuildDurationRenderer;
use App\Reporting\Renderer\FailedBuildListRenderer;
use App\Reporting\Renderer\FailureRateRenderer;
use App\Reporting\Renderer\ReviewLatencyRenderer;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use InvalidArgumentException;

#[Signature(
    'devpulse:report'
    . ' {month : Y-m 格式（例如 2026-04）}'
    . ' {--group= : group slug，必填}'
    . ' {--output= : 輸出檔路徑（不指定則印到 stdout）}',
)]
#[Description('產出指定 group 與月份的 markdown 月報')]
class ReportCommand extends Command
{
    public function __construct(
        private readonly BuildFailureRateQuery $failureRateQuery,
        private readonly ReviewLatencyQuery $reviewLatencyQuery,
        private readonly DailyBuildDurationQuery $dailyBuildDurationQuery,
        private readonly FailedBuildListQuery $failedBuildListQuery,
        private readonly FailureRateRenderer $failureRateRenderer,
        private readonly ReviewLatencyRenderer $reviewLatencyRenderer,
        private readonly DailyBuildDurationRenderer $dailyBuildDurationRenderer,
        private readonly FailedBuildListRenderer $failedBuildListRenderer,
    ) {
        parent::__construct();
    }

    public function handle(): int
    {
        $monthRaw = (string)$this->argument('month');
        $groupSlug = $this->option('group');
        $output = $this->option('output');

        if (!is_string($groupSlug) || $groupSlug === '') {
            $this->error('--group 為必填，請指定 group slug');

            return self::FAILURE;
        }

        try {
            $month = MonthRange::fromString($monthRaw);
        } catch (InvalidArgumentException $e) {
            $this->error("month 格式錯誤：{$e->getMessage()}（預期 Y-m，例如 2026-04）");

            return self::FAILURE;
        }

        $group = Group::query()->where('slug', $groupSlug)->first();
        if ($group === null) {
            $this->error("group `{$groupSlug}` 不存在");

            return self::FAILURE;
        }

        $markdown = $this->buildReport($group, $month, $monthRaw);

        if (is_string($output) && $output !== '') {
            file_put_contents($output, $markdown);
            $this->info("已寫入 {$output}");
        } else {
            foreach (explode("\n", $markdown) as $line) {
                $this->line($line);
            }
        }

        return self::SUCCESS;
    }

    private function buildReport(Group $group, MonthRange $month, string $monthLabel): string
    {
        $failureRate = $this->failureRateQuery->run($group, $month);
        $reviewLatency = $this->reviewLatencyQuery->run($group, $month);
        $dailyDuration = $this->dailyBuildDurationQuery->run($group, $month);
        $failedBuilds = $this->failedBuildListQuery->run($group, $month);

        $sections = [
            "# devpulse 月報：{$group->slug} / {$monthLabel}",
            '',
            $this->failureRateRenderer->render($failureRate),
            $this->reviewLatencyRenderer->render($reviewLatency),
            $this->dailyBuildDurationRenderer->render($dailyDuration),
            $this->failedBuildListRenderer->render($failedBuilds),
        ];

        return implode("\n", $sections);
    }
}
