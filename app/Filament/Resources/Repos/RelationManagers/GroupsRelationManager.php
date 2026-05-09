<?php

declare(strict_types=1);

namespace App\Filament\Resources\Repos\RelationManagers;

use App\Filament\Resources\Groups\GroupResource;
use App\Models\Group;
use Filament\Actions\AttachAction;
use Filament\Actions\BulkActionGroup;
use Filament\Actions\DetachAction;
use Filament\Actions\DetachBulkAction;
use Filament\Resources\RelationManagers\RelationManager;
use Filament\Schemas\Schema;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;

class GroupsRelationManager extends RelationManager
{
    protected static string $relationship = 'groups';

    public function form(Schema $schema): Schema
    {
        return $schema->components([]);
    }

    public function table(Table $table): Table
    {
        return $table
            ->recordTitleAttribute('slug')
            ->columns([
                TextColumn::make('slug')
                    ->searchable()
                    ->sortable()
                    ->url(fn (Group $record): string => GroupResource::getUrl('edit', ['record' => $record])),
                TextColumn::make('description')
                    ->placeholder('—')
                    ->limit(50),
            ])
            ->defaultSort('slug')
            ->headerActions([
                AttachAction::make()->preloadRecordSelect(),
            ])
            ->recordActions([
                DetachAction::make(),
            ])
            ->toolbarActions([
                BulkActionGroup::make([
                    DetachBulkAction::make(),
                ]),
            ]);
    }
}
