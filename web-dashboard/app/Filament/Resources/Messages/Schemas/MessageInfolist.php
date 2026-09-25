<?php

namespace App\Filament\Resources\Messages\Schemas;

use Filament\Infolists\Components\TextEntry;
use Filament\Schemas\Schema;

class MessageInfolist
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                TextEntry::make('source')
                    ->label('Canal')
                    ->badge(),
                TextEntry::make('client.name')
                    ->label('Cliente')
                    ->placeholder('-'),
                TextEntry::make('external_id')
                    ->label('ID externo')
                    ->copyable(),
                TextEntry::make('processed')
                    ->label('Procesado')
                    ->badge()
                    ->formatStateUsing(fn ($state): string => $state ? 'Sí' : 'No')
                    ->color(fn ($state): string => $state ? 'success' : 'warning'),
                TextEntry::make('created_at')
                    ->label('Recibido')
                    ->dateTime('Y-m-d H:i:s')
                    ->placeholder('-'),
                TextEntry::make('content')
                    ->label('Contenido original')
                    ->columnSpanFull()
                    ->extraAttributes(['class' => 'whitespace-pre-wrap']),
            ]);
    }
}
