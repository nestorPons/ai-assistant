<?php

namespace App\Filament\Resources\Messages\Pages;

use App\Filament\Resources\Messages\MessageResource;
use App\Services\AuditLogger;
use Filament\Resources\Pages\ViewRecord;

class ViewMessage extends ViewRecord
{
    protected static string $resource = MessageResource::class;

    public function mount(int|string $record): void
    {
        parent::mount($record);

        app(AuditLogger::class)->log(
            action: 'message.viewed',
            subjectType: $this->getRecord()::class,
            subjectId: $this->getRecord()->getKey(),
        );
    }
}
