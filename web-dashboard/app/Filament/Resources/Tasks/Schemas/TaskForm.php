<?php

namespace App\Filament\Resources\Tasks\Schemas;

use Filament\Forms\Components\DatePicker;
use Filament\Forms\Components\MarkdownEditor;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\ToggleButtons;
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
                ToggleButtons::make('status')
                    ->label('Estado')
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
                    ])
                    ->inline()
                    ->required(),
                DatePicker::make('due_date')
                    ->label('Fecha límite')
                    ->native(false)
                    ->placeholder('Sin fecha'),
                MarkdownEditor::make('spec_md')
                    ->label('spec.md')
                    ->helperText('Especificación de la tarea generada automáticamente; editable.')
                    ->columnSpanFull(),
            ]);
    }
}
