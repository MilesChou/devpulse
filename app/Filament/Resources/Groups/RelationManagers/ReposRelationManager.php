<?php

declare(strict_types=1);

namespace App\Filament\Resources\Groups\RelationManagers;

use App\Filament\Resources\Repos\RepoResource;
use App\Models\Repo;
use Filament\Actions\AttachAction;
use Filament\Actions\BulkActionGroup;
use Filament\Actions\DetachAction;
use Filament\Actions\DetachBulkAction;
use Filament\Resources\RelationManagers\RelationManager;
use Filament\Schemas\Schema;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;

class ReposRelationManager extends RelationManager
{
    protected static string $relationship = 'repos';

    public function form(Schema $schema): Schema
    {
        // 不在 group 內建立／編輯 repo，repo 有自己的 resource。
        return $schema->components([]);
    }

    public function table(Table $table): Table
    {
        return $table
            ->recordTitleAttribute('name')
            ->columns([
                TextColumn::make('name')
                    ->label('Repo')
                    ->searchable()
                    ->sortable()
                    ->url(fn (Repo $record): string => RepoResource::getUrl('edit', ['record' => $record])),
                TextColumn::make('human_signals')
                    ->label('Signals')
                    ->state(fn (Repo $record): int => count($record->human_signals))
                    ->badge()
                    ->color(fn (int $state): string => $state > 0 ? 'success' : 'gray'),
                TextColumn::make('groups_count')
                    ->label('In groups')
                    ->counts('groups')
                    ->badge()
                    ->tooltip('此 repo 共屬於幾個 group'),
            ])
            ->defaultSort('name')
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
