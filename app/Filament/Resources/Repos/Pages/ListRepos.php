<?php

declare(strict_types=1);

namespace App\Filament\Resources\Repos\Pages;

use App\Filament\Resources\Repos\RepoResource;
use Filament\Actions\CreateAction;
use Filament\Resources\Pages\ListRecords;

class ListRepos extends ListRecords
{
    protected static string $resource = RepoResource::class;

    protected function getHeaderActions(): array
    {
        return [
            CreateAction::make(),
        ];
    }
}
