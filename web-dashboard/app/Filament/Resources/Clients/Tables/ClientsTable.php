<?php

namespace App\Filament\Resources\Clients\Tables;

use Filament\Actions\EditAction;
use Filament\Tables\Columns\IconColumn;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Filters\TernaryFilter;
use Filament\Tables\Table;

class ClientsTable
{
    public static function configure(Table $table): Table
    {
        return $table
            ->columns([
                TextColumn::make('name')
                    ->label('Nombre')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('source')
                    ->label('Canal')
                    ->badge()
                    ->sortable(),
                TextColumn::make('identifier')
                    ->label('Identificador')
                    ->searchable(),
                IconColumn::make('tracked')
                    ->label('Rastreado')
                    ->boolean(),
                IconColumn::make('active')
                    ->label('Activo')
                    ->boolean(),
                TextColumn::make('created_at')
                    ->label('Creado')
                    ->dateTime('Y-m-d H:i')
                    ->sortable()
                    ->toggleable(),
            ])
            ->filters([
                SelectFilter::make('source')
                    ->label('Canal')
                    ->options([
                        'gmail' => 'Gmail',
                        'telegram' => 'Telegram',
                        'whatsapp' => 'WhatsApp',
                    ]),
                TernaryFilter::make('tracked')
                    ->label('Rastreado'),
                TernaryFilter::make('active')
                    ->label('Activo'),
            ])
            ->recordActions([
                EditAction::make(),
            ])
            ->defaultSort('created_at', 'desc');
    }
}
