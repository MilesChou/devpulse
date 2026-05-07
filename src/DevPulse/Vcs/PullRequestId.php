<?php

declare(strict_types=1);

namespace DevPulse\Vcs;

use InvalidArgumentException;
use Stringable;

final readonly class PullRequestId implements Stringable
{
    public function __construct(public string $value)
    {
        if ($value === '') {
            throw new InvalidArgumentException('PullRequestId must not be empty');
        }
    }

    public function __toString(): string
    {
        return $this->value;
    }
}
