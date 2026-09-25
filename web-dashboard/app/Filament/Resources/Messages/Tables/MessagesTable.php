<?php

namespace App\Filament\Resources\Messages\Tables;

use Filament\Actions\ViewAction;
use Filament\Tables\Columns\IconColumn;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Filters\TernaryFilter;
use Filament\Tables\Table;

class MessagesTable
{
    public static function configure(Table $table): Table
    {
        return $table
            ->columns([
                TextColumn::make('created_at')
                    ->label('Recibido')
                    ->dateTime('Y-m-d H:i')
                    ->sortable(),
                TextColumn::make('source')
                    ->label('Canal')
                    ->badge()
                    ->sortable(),
                TextColumn::make('client.name')
                    ->label('Cliente')
                    ->placeholder('-')
                    ->searchable(),
                IconColumn::make('processed')
                    ->label('Procesado')
                    ->boolean(),
                TextColumn::make('content')
                    ->label('Contenido')
                    ->limit(80)
                    ->wrap(),
            ])
            ->filters([
                SelectFilter::make('source')
                    ->label('Canal')
                    ->options([
                        'gmail' => 'Gmail',
                        'telegram' => 'Telegram',
                        'whatsapp' => 'WhatsApp',
                    ]),
                TernaryFilter::make('processed')
                    ->label('Procesado'),
            ])
            ->recordActions([
                ViewAction::make(),
            ])
            ->defaultSort('created_at', 'desc');
    }
}
