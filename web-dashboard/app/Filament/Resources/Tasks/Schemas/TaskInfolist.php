<?php

namespace App\Filament\Resources\Tasks\Schemas;

use App\Filament\Resources\Messages\MessageResource;
use App\Models\Kos\Task;
use Filament\Infolists\Components\TextEntry;
use Filament\Schemas\Schema;

class TaskInfolist
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                TextEntry::make('subject')
                    ->label('Tema'),
                TextEntry::make('client.name')
                    ->label('Cliente')
                    ->placeholder('-'),
                TextEntry::make('title')
                    ->label('Título')
                    ->columnSpanFull(),
                TextEntry::make('description')
                    ->label('Descripción')
                    ->columnSpanFull(),
                TextEntry::make('priority')
                    ->label('Prioridad')
                    ->badge(),
                TextEntry::make('status')
                    ->label('Estado')
                    ->badge(),
                TextEntry::make('estimated_hours')
                    ->label('Horas estimadas')
                    ->numeric()
                    ->placeholder('-'),
                TextEntry::make('due_date')
                    ->label('Fecha límite')
                    ->date('Y-m-d')
                    ->placeholder('Sin fecha'),
                TextEntry::make('ai_confidence')
                    ->label('Confianza IA')
                    ->formatStateUsing(fn ($state): string => number_format(((float) $state) * 100, 0).'%'),
                TextEntry::make('specifications')
                    ->label('Especificaciones')
                    ->placeholder('-')
                    ->columnSpanFull()
                    ->formatStateUsing(fn ($state): ?string => filled($state)
                        ? json_encode($state, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES)
                        : null),
                TextEntry::make('message_id')
                    ->label('Mensaje original')
                    ->placeholder('-')
                    ->url(fn (Task $record): ?string => $record->message_id
                        ? MessageResource::getUrl('view', ['record' => $record->message_id])
                        : null),
                TextEntry::make('created_at')
                    ->label('Creada')
                    ->dateTime('Y-m-d H:i')
                    ->placeholder('-'),
                TextEntry::make('updated_at')
                    ->label('Actualizada')
                    ->dateTime('Y-m-d H:i')
                    ->placeholder('-'),
            ]);
    }
}
