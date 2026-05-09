<?php

declare(strict_types=1);

namespace App\Filament\Resources\Members\Schemas;

use Filament\Forms\Components\TextInput;
use Filament\Schemas\Schema;

class MemberForm
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                TextInput::make('display_name')
                    ->required()
                    ->maxLength(64),
                TextInput::make('github_account')
                    ->required()
                    ->maxLength(64)
                    ->unique(ignoreRecord: true),
            ]);
    }
}
