<?php

declare(strict_types=1);

namespace App\Domain\Shared;

use InvalidArgumentException;

final readonly class RepoId
{
    public function __construct(public int $value)
    {
        if ($value < 1) {
            throw new InvalidArgumentException('RepoId must be >= 1');
        }
    }

    public function toInt(): int
    {
        return $this->value;
    }
}
