<?php

declare(strict_types=1);

namespace App\Filament\Resources\Groups\Schemas;

use App\Models\Group;
use Filament\Forms\Components\TextInput;
use Filament\Schemas\Schema;

class GroupForm
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                TextInput::make('slug')
                    ->required()
                    ->maxLength(64)
                    ->regex(Group::SLUG_PATTERN)
                    ->helperText('小寫英數與 dash，例如 my-team')
                    ->unique(ignoreRecord: true),
                TextInput::make('description')
                    ->maxLength(255),
            ]);
    }
}
