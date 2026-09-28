<?php

namespace App\Filament\Resources\Tasks\Tables;

use App\Filament\Resources\Tasks\Pages\EditTask;
use App\Models\Kos\Task;
use Filament\Actions\EditAction;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Table;

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
                    ->sortable(),
                TextColumn::make('priority')
                    ->label('Prioridad')
                    ->badge()
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
                    ->toggleable(),
                TextColumn::make('created_at')
                    ->label('Creada')
                    ->dateTime('Y-m-d H:i')
                    ->sortable()
                    ->toggleable(),
            ])
            ->filters([
                SelectFilter::make('status')
                    ->label('Estado')
                    ->options([
                        'pending' => 'Pendiente',
                        'in_progress' => 'En progreso',
                        'needs_review' => 'Revisión',
                        'completed' => 'Completada',
                        'discarded' => 'Descartada',
                    ]),
                SelectFilter::make('priority')
                    ->label('Prioridad')
                    ->options([
                        'low' => 'Baja',
                        'medium' => 'Media',
                        'high' => 'Alta',
                    ]),
            ])
            ->recordUrl(fn (Task $record): string => EditTask::getUrl(['record' => $record]))
            ->recordActions([
                EditAction::make(),
            ])
            ->poll('5s')
            ->defaultSort('created_at', 'desc');
    }
}
