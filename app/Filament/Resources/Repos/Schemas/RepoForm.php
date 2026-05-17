<?php

declare(strict_types=1);

namespace App\Filament\Resources\Repos\Schemas;

use App\Support\HumanSignalCategoryOptions;
use Closure;
use DevPulse\Shared\RepoFullName;
use Filament\Forms\Components\Repeater;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\TextInput;
use Filament\Schemas\Components\Section;
use Filament\Schemas\Schema;

class RepoForm
{
    public static function configure(Schema $schema): Schema
    {
        $categoryOptions = HumanSignalCategoryOptions::all();

        return $schema
            ->components([
                TextInput::make('slug')
                    ->required()
                    ->maxLength(64)
                    ->placeholder('devpulse')
                    ->helperText('ULID 的人類好讀版，唯一短代號')
                    ->unique(ignoreRecord: true)
                    ->columnSpan(1),
                Select::make('type')
                    ->required()
                    ->options([
                        'github' => 'GitHub',
                        'gitlab' => 'GitLab',
                    ])
                    ->default('github')
                    ->columnSpan(1),
                TextInput::make('name')
                    ->required()
                    ->maxLength(255)
                    ->placeholder('owner/name')
                    ->rules([self::repoNameRule()])
                    ->helperText('owner/name 格式，例如 your-org/your-repo')
                    ->columnSpanFull(),
                TextInput::make('url')
                    ->required()
                    ->maxLength(500)
                    ->placeholder('git@github.com:owner/repo.git')
                    ->helperText('git clone URL')
                    ->columnSpanFull(),
                Section::make('human_signals')
                    ->description('classifier 用來把失敗 build 歸類為 human / infra 的關鍵字。新增分類請改 config/devpulse.php。')
                    ->schema([
                        Repeater::make('human_signals')
                            ->hiddenLabel()
                            ->schema([
                                Select::make('category')
                                    ->options($categoryOptions)
                                    ->searchable()
                                    ->required(),
                                TextInput::make('pattern')
                                    ->required()
                                    ->maxLength(255)
                                    ->placeholder('例如 PHPCS:'),
                            ])
                            ->columns(2)
                            ->reorderable()
                            ->addActionLabel('＋ 新增 signal')
                            ->itemLabel(
                                /** @param array<string, mixed> $state */
                                function (array $state): ?string {
                                    $category = $state['category'] ?? null;
                                    $pattern = $state['pattern'] ?? null;
                                    if (! is_string($category) || ! is_string($pattern)) {
                                        return null;
                                    }

                                    return sprintf('[%s] %s', $category, $pattern);
                                },
                            )
                            ->collapsible()
                            ->defaultItems(0),
                    ])
                    ->columnSpanFull(),
            ])
            ->columns(2);
    }

    private static function repoNameRule(): Closure
    {
        return function (string $attribute, mixed $value, Closure $fail): void {
            if (! is_string($value) || ! RepoFullName::isValid($value)) {
                $fail('name 必須是 owner/name 格式');
            }
        };
    }
}
