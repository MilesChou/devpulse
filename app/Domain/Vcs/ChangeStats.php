<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use InvalidArgumentException;

final readonly class ChangeStats
{
    public function __construct(
        public int $additions,
        public int $deletions,
    ) {
        if ($additions < 0) {
            throw new InvalidArgumentException('additions must not be negative');
        }
        if ($deletions < 0) {
            throw new InvalidArgumentException('deletions must not be negative');
        }
    }

    public function total(): int
    {
        return $this->additions + $this->deletions;
    }
}
