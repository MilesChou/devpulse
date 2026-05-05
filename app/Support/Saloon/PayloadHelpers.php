<?php

declare(strict_types=1);

namespace App\Support\Saloon;

/**
 * Saloon Response::json() 回傳 mixed，需要明確 narrow 為精確型別才能傳給
 * VO::fromXxxRaw()。這個 helper 集中處理 narrow 邏輯，避免每個 provider 重複。
 */
final class PayloadHelpers
{
    /**
     * @param mixed $value
     * @return array<string, mixed>
     */
    public static function stringKeyedArray($value): array
    {
        if (! is_array($value)) {
            return [];
        }

        $result = [];
        foreach ($value as $key => $item) {
            if (is_string($key)) {
                $result[$key] = $item;
            }
        }

        return $result;
    }

    /**
     * @param mixed $value
     * @return list<array<string, mixed>>
     */
    public static function listOfArrays($value): array
    {
        if (! is_array($value)) {
            return [];
        }

        $result = [];
        foreach ($value as $item) {
            if (is_array($item)) {
                $result[] = self::stringKeyedArray($item);
            }
        }

        return $result;
    }
}
