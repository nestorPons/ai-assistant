<?php

namespace App\Listeners;

use App\Services\AuditLogger;
use Illuminate\Auth\Events\Logout;

class RecordLogout
{
    public function __construct(private AuditLogger $audit) {}

    public function handle(Logout $event): void
    {
        $this->audit->log(
            action: 'logout',
            subjectType: $event->user?->getMorphClass(),
            subjectId: $event->user?->getAuthIdentifier(),
            user: $event->user?->email,
        );
    }
}
