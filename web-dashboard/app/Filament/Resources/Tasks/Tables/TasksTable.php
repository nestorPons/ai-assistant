<?php

namespace App\Filament\Resources\Tasks\Tables;

use App\Filament\Resources\Tasks\Pages\EditTask;
use App\Models\TskHub\Task;
use Filament\Actions\EditAction;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\ToggleButtons;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Enums\FiltersLayout;
use Filament\Tables\Filters\Filter;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Table;
use Illuminate\Database\Eloquent\Builder;

class TasksTable
{
    public static function configure(Table $table): Table
    {
        return $table
            ->columns([
                TextColumn::make('subject')
                    ->label('Tema')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('title')
                    ->label('Título')
                    ->searchable()
                    ->limit(50),
                TextColumn::make('status')
                    ->label('Estado')
                    ->badge()
                    ->colors([
                        'warning' => 'pending',
                        'info' => 'in_progress',
                        'gray' => 'needs_review',
                        'success' => 'completed',
                        'danger' => 'discarded',
                    ])
                    ->formatStateUsing(fn (string $state): string => match ($state) {
                        'pending' => 'Pendiente',
                        'in_progress' => 'En progreso',
                        'needs_review' => 'Revisión',
                        'completed' => 'Completada',
                        'discarded' => 'Descartada',
                        default => $state,
                    })
                    ->sortable(),
                TextColumn::make('priority')
                    ->label('Prioridad')
                    ->badge()
                    ->colors([
                        'gray' => 'low',
                        'warning' => 'medium',
                        'danger' => 'high',
                    ])
                    ->formatStateUsing(fn (string $state): string => match ($state) {
                        'low' => 'Baja',
                        'medium' => 'Media',
                        'high' => 'Alta',
                        default => $state,
                    })
                    ->sortable(),
                TextColumn::make('ai_confidence')
                    ->label('Confianza')
                    ->formatStateUsing(fn ($state): string => number_format(((float) $state) * 100, 0).'%'),
                TextColumn::make('due_date')
                    ->label('Fecha límite')
                    ->date('Y-m-d')
                    ->placeholder('-')
                    ->sortable(),
                TextColumn::make('client.name')
                    ->label('Cliente')
                    ->placeholder('-')
                    ->searchable()
                    ->toggleable(),
                TextColumn::make('created_at')
                    ->label('Creada')
                    ->dateTime('Y-m-d H:i')
                    ->sortable()
                    ->toggleable(),
            ])
            ->filters([
                Filter::make('status')
                    ->label('Estado')
                    ->columnSpan(2)
                    ->form([
                        ToggleButtons::make('value')
                            ->hiddenLabel()
                            ->inline()
                            ->extraAttributes(['class' => 'task-status-filter'])
                            ->options([
                                'pending' => 'Pendiente',
                                'in_progress' => 'En progreso',
                                'needs_review' => 'Revisión',
                                'completed' => 'Completada',
                                'discarded' => 'Descartada',
                            ])
                            ->colors([
                                'pending' => 'warning',
                                'in_progress' => 'info',
                                'needs_review' => 'gray',
                                'completed' => 'success',
                                'discarded' => 'danger',
                            ]),
                    ])
                    ->query(fn (Builder $query, array $data): Builder => $query->when(
                        $data['value'] ?? null,
                        fn (Builder $query, string $status): Builder => $query->where('status', $status),
                    )),
                SelectFilter::make('priority')
                    ->modifyFormFieldUsing(
                        fn (Select $field): Select => $field->hiddenLabel(),
                    )
                    ->columnSpan(1)
                    ->indicateUsing(fn (): array => [])
                    ->options([
                        'low' => 'Baja',
                        'medium' => 'Media',
                        'high' => 'Alta',
                    ]),
            ])
            ->filtersLayout(FiltersLayout::AboveContent)
            ->filtersFormColumns(3)
            ->deferFilters(false)
            ->searchPlaceholder('Tema, título o cliente…')
            ->recordUrl(fn (Task $record): string => EditTask::getUrl(['record' => $record]))
            ->recordActions([
                EditAction::make(),
            ])
            ->poll('5s')
            ->defaultSort('created_at', 'desc');
    }
}
