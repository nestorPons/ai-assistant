<?php

namespace App\Filament\Resources\Clients\Schemas;

use App\Models\Kos\Client;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Schemas\Components\Utilities\Get;
use Filament\Schemas\Schema;
use Illuminate\Validation\Rule;

class ClientForm
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                Select::make('source')
                    ->label('Canal')
                    ->options([
                        'gmail' => 'Gmail',
                        'telegram' => 'Telegram',
                        'whatsapp' => 'WhatsApp',
                    ])
                    ->required()
                    ->disabledOn('edit')
                    ->dehydrated(),
                TextInput::make('identifier')
                    ->label('Identificador')
                    ->helperText('Email, teléfono o username del canal.')
                    ->required()
                    ->maxLength(255)
                    ->disabledOn('edit')
                    ->dehydrated()
                    ->rule(fn (Get $get, ?Client $record) => Rule::unique('clients', 'identifier')
                        ->where(fn ($query) => $query->where('source', $get('source')))
                        ->ignore($record?->getKey())),
                TextInput::make('name')
                    ->label('Nombre')
                    ->required()
                    ->maxLength(255),
                Toggle::make('tracked')
                    ->label('Rastreado')
                    ->helperText('Incluido en la lista blanca.')
                    ->default(true),
                Toggle::make('active')
                    ->label('Activo')
                    ->default(true),
            ]);
    }
}
