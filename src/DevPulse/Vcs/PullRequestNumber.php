<?php

declare(strict_types=1);

namespace DevPulse\Vcs;

use InvalidArgumentException;

final readonly class PullRequestNumber
{
    public function __construct(public int $value)
    {
        if ($value < 1) {
            throw new InvalidArgumentException('PullRequestNumber must be >= 1');
        }
    }

    public function toInt(): int
    {
        return $this->value;
    }
}
