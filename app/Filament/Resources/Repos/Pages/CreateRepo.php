<?php

declare(strict_types=1);

namespace App\Filament\Resources\Repos\Pages;

use App\Filament\Resources\Repos\RepoResource;
use Filament\Resources\Pages\CreateRecord;

class CreateRepo extends CreateRecord
{
    protected static string $resource = RepoResource::class;
}
