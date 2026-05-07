<?php

declare(strict_types=1);

namespace DevPulse\Vcs;

enum Platform: string
{
    case GitHub = 'github';
    case GitLab = 'gitlab';
}
