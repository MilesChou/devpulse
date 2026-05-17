<?php

declare(strict_types=1);

namespace DevPulse\Shared;

use InvalidArgumentException;
use Stringable;

final readonly class RepoId implements Stringable
{
    public function __construct(public string $value)
    {
        if (! preg_match('/^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26}$/', $value)) {
            throw new InvalidArgumentException("RepoId must be a 26-char ULID (got `{$value}`)");
        }
    }

    public function toString(): string
    {
        return $this->value;
    }

    public function __toString(): string
    {
        return $this->value;
    }
}
