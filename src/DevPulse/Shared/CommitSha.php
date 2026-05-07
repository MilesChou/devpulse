<?php

declare(strict_types=1);

namespace DevPulse\Shared;

use InvalidArgumentException;
use Stringable;

final class CommitSha implements Stringable
{
    public readonly string $value;

    public function __construct(string $value)
    {
        if (strlen($value) < 7) {
            throw new InvalidArgumentException('commitSha must be at least 7 characters');
        }
        $this->value = $value;
    }

    public function __toString(): string
    {
        return $this->value;
    }
}
