<?php

declare(strict_types=1);

namespace App\Filament\Resources\Groups\RelationManagers;

use App\Filament\Resources\Members\MemberResource;
use App\Models\Member;
use Filament\Actions\AttachAction;
use Filament\Actions\BulkActionGroup;
use Filament\Actions\DetachAction;
use Filament\Actions\DetachBulkAction;
use Filament\Resources\RelationManagers\RelationManager;
use Filament\Schemas\Schema;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;

class MembersRelationManager extends RelationManager
{
    protected static string $relationship = 'members';

    public function form(Schema $schema): Schema
    {
        return $schema->components([]);
    }

    public function table(Table $table): Table
    {
        return $table
            ->recordTitleAttribute('display_name')
            ->columns([
                TextColumn::make('display_name')
                    ->label('Display name')
                    ->searchable()
                    ->sortable()
                    ->url(fn (Member $record): string => MemberResource::getUrl('edit', ['record' => $record])),
                TextColumn::make('github_account')
                    ->label('GitHub')
                    ->searchable()
                    ->copyable()
                    ->prefix('@'),
                TextColumn::make('groups_count')
                    ->label('In groups')
                    ->counts('groups')
                    ->badge()
                    ->tooltip('此成員共屬於幾個 group'),
            ])
            ->defaultSort('display_name')
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
