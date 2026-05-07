<?php

declare(strict_types=1);

namespace DevPulse\Shared;

use InvalidArgumentException;
use Stringable;

final class RepoFullName implements Stringable
{
    public readonly string $owner;
    public readonly string $name;

    public function __construct(string $value)
    {
        $parts = explode('/', $value, 2);
        if (count($parts) !== 2 || $parts[0] === '' || $parts[1] === '') {
            throw new InvalidArgumentException("repoFullName must be owner/name format (got `{$value}`)");
        }
        $this->owner = $parts[0];
        $this->name = $parts[1];
    }

    public function __toString(): string
    {
        return "{$this->owner}/{$this->name}";
    }
}
