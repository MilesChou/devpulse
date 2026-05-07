<?php

declare(strict_types=1);

namespace App\Aggregation;

use InvalidArgumentException;

/**
 * 依設定把 PR 改動行數分入對應的 size bucket。
 *
 * 設定格式（來自 config/devpulse.php pr_size_buckets）：
 *   ['XS' => 50, 'S' => 200, 'M' => 500, 'L' => 1000, 'XL' => null]
 * 最後一個桶的上限為 null，代表無上限（包含所有超過前一桶的值）。
 */
final class PrSizeBucket
{
    /** @var array<string, int|null> */
    private array $buckets;

    /**
     * @param array<string, int|null> $buckets
     */
    public function __construct(array $buckets)
    {
        if (empty($buckets)) {
            throw new InvalidArgumentException('pr_size_buckets config must not be empty');
        }

        $this->buckets = $buckets;
    }

    public static function fromConfig(): self
    {
        /** @var array<string, int|null> $cfg */
        $cfg = config('devpulse.pr_size_buckets');

        return new self($cfg);
    }

    /**
     * 依 PR 改動總行數回傳對應的 bucket 名稱。
     */
    public function classify(int $totalChangedLines): string
    {
        foreach ($this->buckets as $name => $limit) {
            if ($limit === null || $totalChangedLines < $limit) {
                return $name;
            }
        }

        return (string)array_key_last($this->buckets);
    }
}
