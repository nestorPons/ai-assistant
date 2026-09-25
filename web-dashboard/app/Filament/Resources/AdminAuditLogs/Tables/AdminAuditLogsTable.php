<?php

namespace App\Filament\Resources\AdminAuditLogs\Tables;

use Filament\Actions\ViewAction;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Table;

class AdminAuditLogsTable
{
    public static function configure(Table $table): Table
    {
        return $table
            ->columns([
                TextColumn::make('created_at')
                    ->label('Fecha')
                    ->dateTime('Y-m-d H:i:s')
                    ->sortable(),
                TextColumn::make('user')
                    ->label('Usuario')
                    ->searchable(),
                TextColumn::make('action')
                    ->label('Acción')
                    ->badge()
                    ->searchable(),
                TextColumn::make('subject_type')
                    ->label('Entidad')
                    ->placeholder('-')
                    ->toggleable(),
                TextColumn::make('subject_id')
                    ->label('ID entidad')
                    ->placeholder('-')
                    ->toggleable(isToggledHiddenByDefault: true),
                TextColumn::make('ip')
                    ->label('IP')
                    ->placeholder('-')
                    ->toggleable(),
            ])
            ->filters([
                SelectFilter::make('action')
                    ->label('Acción')
                    ->options([
                        'login' => 'Login',
                        'logout' => 'Logout',
                        'client.created' => 'Cliente creado',
                        'client.updated' => 'Cliente actualizado',
                        'task.created' => 'Tarea creada',
                        'task.updated' => 'Tarea actualizada',
                        'message.viewed' => 'Mensaje abierto',
                    ]),
            ])
            ->recordActions([
                ViewAction::make(),
            ])
            ->defaultSort('created_at', 'desc');
    }
}
