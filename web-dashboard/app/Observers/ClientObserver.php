<?php

namespace App\Observers;

use App\Models\Kos\Client;
use App\Services\AuditLogger;

class ClientObserver
{
    public function __construct(private AuditLogger $audit) {}

    public function created(Client $client): void
    {
        $this->audit->log('client.created', Client::class, $client->getKey(), [
            'attributes' => $client->only(['source', 'identifier', 'name', 'tracked', 'active']),
        ]);
    }

    public function updated(Client $client): void
    {
        $this->audit->log('client.updated', Client::class, $client->getKey(), [
            'changes' => $client->getChanges(),
        ]);
    }
}
