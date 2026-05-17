<?php

declare(strict_types=1);

namespace DevPulse\Shared;

use DateMalformedStringException;
use DateTimeImmutable;
use DateTimeZone;
use InvalidArgumentException;

final class UtcTimestamp
{
    /**
     * Parse a required ISO-8601 timestamp from a payload field.
     *
     * setTimezone(UTC) normalizes the timezone object's name (e.g. "Z" or "+00:00" → "UTC");
     * the instant itself is not shifted.
     *
     * @param array<string, mixed> $raw
     * @throws DateMalformedStringException
     */
    public static function required(array $raw, string $key, string $missingMessage): DateTimeImmutable
    {
        $value = $raw[$key] ?? null;
        if (! is_string($value) || $value === '') {
            throw new InvalidArgumentException($missingMessage);
        }

        return new DateTimeImmutable($value)->setTimezone(new DateTimeZone('UTC'));
    }

    /**
     * Parse an optional ISO-8601 timestamp from a payload field; returns null when absent or empty.
     *
     * @param array<string, mixed> $raw
     * @throws DateMalformedStringException
     */
    public static function optional(array $raw, string $key): ?DateTimeImmutable
    {
        $value = $raw[$key] ?? null;
        if (! is_string($value) || $value === '') {
            return null;
        }

        return new DateTimeImmutable($value)->setTimezone(new DateTimeZone('UTC'));
    }
}
