<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

final readonly class MonthComparison
{
    public ComparisonDirection $direction;
    public float $delta;

    public function __construct(
        public float $current,
        public ?float $previous,
    ) {
        if ($previous === null) {
            $this->direction = ComparisonDirection::Neutral;
            $this->delta = 0.0;
        } elseif ($current > $previous) {
            $this->direction = ComparisonDirection::Up;
            $this->delta = $current - $previous;
        } elseif ($current < $previous) {
            $this->direction = ComparisonDirection::Down;
            $this->delta = $previous - $current;
        } else {
            $this->direction = ComparisonDirection::Neutral;
            $this->delta = 0.0;
        }
    }

    /**
     * 格式化成「↑ +2.00%」或「↓ -1.50%」或「→」樣式的字串。
     */
    public function format(int $decimals = 2, string $suffix = '%'): string
    {
        if ($this->direction === ComparisonDirection::Neutral) {
            return $this->direction->value;
        }

        $sign = $this->direction === ComparisonDirection::Up ? '+' : '-';
        $value = number_format($this->delta, $decimals);

        return "{$this->direction->value} {$sign}{$value}{$suffix}";
    }
}
