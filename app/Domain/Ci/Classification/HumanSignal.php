<?php

declare(strict_types=1);

namespace App\Domain\Ci\Classification;

use InvalidArgumentException;

/**
 * 一條 human signal 規則：當 build log 包含 pattern 時，把 build 歸類為 category。
 *
 * 例如 { category: "lint", pattern: "PHPCS:" }：log 含 "PHPCS:" 就分到 lint 類。
 */
final readonly class HumanSignal
{
    public function __construct(
        public string $category,
        public string $pattern,
    ) {
        if ($category === '') {
            throw new InvalidArgumentException('category must not be empty');
        }
        if ($pattern === '') {
            throw new InvalidArgumentException('pattern must not be empty');
        }
    }

    /**
     * 從 DB JSON / config array 還原為 VO。
     *
     * @param array{category: string, pattern: string} $raw
     */
    public static function fromArray(array $raw): self
    {
        return new self(category: $raw['category'], pattern: $raw['pattern']);
    }
}
