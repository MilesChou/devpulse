<?php

declare(strict_types=1);

namespace App\Http\Controllers\Web;

use App\Http\Controllers\Controller;
use App\Services\Web\PrErrorRateQuery;
use App\Services\Web\PrIterationQuery;
use App\Services\Web\PrLifecycleQuery;
use Carbon\CarbonImmutable;
use Illuminate\Http\Request;
use Illuminate\Support\Number;
use Inertia\Inertia;
use Inertia\Response;

final class DashboardController extends Controller
{
    /**
     * Query string 與 config fallback 的安全邊界。
     */
    private const DAYS_MIN = 1;
    private const DAYS_MAX = 365;
    private const ERROR_RATE_MIN = 0.0;
    private const ERROR_RATE_MAX = 1.0;
    private const ITERATION_MIN = 1.0;
    private const ITERATION_MAX = 100.0;

    public function index(
        Request $request,
        PrErrorRateQuery $errorRate,
        PrIterationQuery $iteration,
        PrLifecycleQuery $lifecycle,
    ): Response {
        $days = $this->resolveInt(
            $request->query('days'),
            $this->configInt('devpulse.dashboard_days', 30),
            self::DAYS_MIN,
            self::DAYS_MAX,
        );

        // 閾值優先序：query string > group thresholds（將來）> config fallback。
        $errorThreshold = $this->resolveFloat(
            $request->query('error_threshold'),
            $this->configFloat('devpulse.thresholds.error_rate', 0.30),
            self::ERROR_RATE_MIN,
            self::ERROR_RATE_MAX,
        );

        $iterationThreshold = $this->resolveFloat(
            $request->query('iteration_threshold'),
            $this->configFloat('devpulse.thresholds.iteration', 3.0),
            self::ITERATION_MIN,
            self::ITERATION_MAX,
        );

        $to = CarbonImmutable::today();
        $from = $to->subDays($days - 1);

        return Inertia::render('Dashboard', [
            'errorRate' => $errorRate->summary($from, $to),
            'iteration' => $iteration->summary($from, $to),
            'lifecycle' => $lifecycle->overallP90($from, $to),
            'thresholds' => [
                'error_rate' => $errorThreshold,
                'iteration' => $iterationThreshold,
            ],
            'range' => [
                'from' => $from->toDateString(),
                'to' => $to->toDateString(),
                'days' => $days,
            ],
        ]);
    }

    private function resolveInt(mixed $raw, int $default, int $min, int $max): int
    {
        $value = is_numeric($raw) ? (int)$raw : $default;
        return (int)Number::clamp($value, $min, $max);
    }

    private function resolveFloat(mixed $raw, float $default, float $min, float $max): float
    {
        $value = is_numeric($raw) ? (float)$raw : $default;
        return (float)Number::clamp($value, $min, $max);
    }

    private function configInt(string $key, int $fallback): int
    {
        $value = config($key);
        return is_numeric($value) ? (int)$value : $fallback;
    }

    private function configFloat(string $key, float $fallback): float
    {
        $value = config($key);
        return is_numeric($value) ? (float)$value : $fallback;
    }
}
