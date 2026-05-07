<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use InvalidArgumentException;
use Stringable;

final readonly class Author implements Stringable
{
    public function __construct(public string $value)
    {
        if ($value === '') {
            throw new InvalidArgumentException('Author must not be empty');
        }
    }

    public function __toString(): string
    {
        return $this->value;
    }
}
