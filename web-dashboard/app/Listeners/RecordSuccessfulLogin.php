<?php

namespace App\Listeners;

use App\Services\AuditLogger;
use Illuminate\Auth\Events\Login;

class RecordSuccessfulLogin
{
    public function __construct(private AuditLogger $audit) {}

    public function handle(Login $event): void
    {
        $this->audit->log(
            action: 'login',
            subjectType: $event->user::class,
            subjectId: $event->user->getAuthIdentifier(),
            user: $event->user->email ?? null,
        );
    }
}
