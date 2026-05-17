<?php

declare(strict_types=1);

namespace App\Filament\Resources\Repos;

use App\Filament\Resources\Repos\Pages\CreateRepo;
use App\Filament\Resources\Repos\Pages\EditRepo;
use App\Filament\Resources\Repos\Pages\ListRepos;
use App\Filament\Resources\Repos\RelationManagers\GroupsRelationManager;
use App\Filament\Resources\Repos\Schemas\RepoForm;
use App\Filament\Resources\Repos\Tables\ReposTable;
use App\Models\Repo;
use BackedEnum;
use Filament\Resources\Resource;
use Filament\Schemas\Schema;
use Filament\Support\Icons\Heroicon;
use Filament\Tables\Table;

class RepoResource extends Resource
{
    protected static ?string $model = Repo::class;

    protected static string|BackedEnum|null $navigationIcon = Heroicon::OutlinedCodeBracket;

    protected static ?string $recordTitleAttribute = 'name';

    public static function form(Schema $schema): Schema
    {
        return RepoForm::configure($schema);
    }

    public static function table(Table $table): Table
    {
        return ReposTable::configure($table);
    }

    public static function getRelations(): array
    {
        return [
            GroupsRelationManager::class,
        ];
    }

    public static function getPages(): array
    {
        return [
            'index' => ListRepos::route('/'),
            'create' => CreateRepo::route('/create'),
            'edit' => EditRepo::route('/{record}/edit'),
        ];
    }
}
