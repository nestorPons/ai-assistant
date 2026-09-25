<?php

namespace App\Filament\Resources\AdminAuditLogs\Schemas;

use Filament\Infolists\Components\TextEntry;
use Filament\Schemas\Schema;

class AdminAuditLogInfolist
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                TextEntry::make('created_at')
                    ->label('Fecha')
                    ->dateTime('Y-m-d H:i:s'),
                TextEntry::make('user')
                    ->label('Usuario'),
                TextEntry::make('action')
                    ->label('Acción')
                    ->badge(),
                TextEntry::make('subject_type')
                    ->label('Entidad')
                    ->placeholder('-'),
                TextEntry::make('subject_id')
                    ->label('ID entidad')
                    ->placeholder('-')
                    ->copyable(),
                TextEntry::make('ip')
                    ->label('IP')
                    ->placeholder('-'),
                TextEntry::make('changes')
                    ->label('Cambios')
                    ->placeholder('-')
                    ->columnSpanFull()
                    ->formatStateUsing(fn ($state): ?string => filled($state)
                        ? json_encode($state, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES)
                        : null),
                TextEntry::make('user_agent')
                    ->label('User-Agent')
                    ->placeholder('-')
                    ->columnSpanFull(),
            ]);
    }
}
