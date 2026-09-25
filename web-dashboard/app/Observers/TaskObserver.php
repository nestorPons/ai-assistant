<?php

namespace App\Observers;

use App\Models\Kos\Task;
use App\Services\AuditLogger;

class TaskObserver
{
    public function __construct(private AuditLogger $audit) {}

    public function created(Task $task): void
    {
        $this->audit->log('task.created', Task::class, $task->getKey(), [
            'attributes' => $task->only(['title', 'subject', 'priority', 'status']),
        ]);
    }

    public function updated(Task $task): void
    {
        $this->audit->log('task.updated', Task::class, $task->getKey(), [
            'changes' => $task->getChanges(),
        ]);
    }
}
