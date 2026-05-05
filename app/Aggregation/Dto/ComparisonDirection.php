<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

enum ComparisonDirection: string
{
    case Up = '↑';
    case Down = '↓';
    case Neutral = '→';
}
