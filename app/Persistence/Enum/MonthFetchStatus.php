<?php

declare(strict_types=1);

namespace App\Persistence\Enum;

enum MonthFetchStatus: string
{
    case Complete = 'complete';
    case Partial = 'partial';
}
