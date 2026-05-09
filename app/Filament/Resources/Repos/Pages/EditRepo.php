<?php

declare(strict_types=1);

namespace App\Filament\Resources\Repos\Pages;

use App\Filament\Resources\Repos\RepoResource;
use Filament\Actions\DeleteAction;
use Filament\Resources\Pages\EditRecord;

class EditRepo extends EditRecord
{
    protected static string $resource = RepoResource::class;

    protected function getHeaderActions(): array
    {
        return [
            DeleteAction::make(),
        ];
    }
}
