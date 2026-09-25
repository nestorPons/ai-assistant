<?php

namespace App\Filament\Resources\Tasks\Schemas;

use Filament\Forms\Components\DatePicker;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Schemas\Schema;

class TaskForm
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->components([
                TextInput::make('subject')
                    ->label('Tema')
                    ->helperText('Proyecto, web, cliente o sistema sobre el que se trabaja.')
                    ->required()
                    ->maxLength(255),
                TextInput::make('title')
                    ->label('Título')
                    ->required()
                    ->maxLength(500),
                Textarea::make('description')
                    ->label('Descripción')
                    ->rows(4)
                    ->columnSpanFull(),
                Select::make('priority')
                    ->label('Prioridad')
                    ->options([
                        'low' => 'Baja',
                        'medium' => 'Media',
                        'high' => 'Alta',
                    ])
                    ->required(),
                Select::make('status')
                    ->label('Estado')
                    ->options([
                        'pending' => 'Pendiente',
                        'in_progress' => 'En progreso',
                        'needs_review' => 'Revisión',
                        'completed' => 'Completada',
                        'discarded' => 'Descartada',
                    ])
                    ->required(),
                TextInput::make('estimated_hours')
                    ->label('Horas estimadas')
                    ->numeric()
                    ->step('0.1')
                    ->placeholder('-'),
                DatePicker::make('due_date')
                    ->label('Fecha límite')
                    ->native(false)
                    ->placeholder('Sin fecha'),
            ]);
    }
}
