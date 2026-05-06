<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use InvalidArgumentException;

final readonly class Author
{
    public function __construct(public string $value)
    {
        if ($value === '') {
            throw new InvalidArgumentException('Author must not be empty');
        }
    }

    public function toString(): string
    {
        return $this->value;
    }
}
