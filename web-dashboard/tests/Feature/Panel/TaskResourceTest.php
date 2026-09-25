<?php

namespace Tests\Feature\Panel;

use App\Filament\Resources\Tasks\Pages\EditTask;
use App\Models\Kos\Task;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Livewire\Livewire;
use Tests\TestCase;

class TaskResourceTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_can_edit_a_task_and_the_change_is_audited(): void
    {
        $admin = User::factory()->admin()->create();
        $task = Task::factory()->create(['status' => 'pending']);
        $this->actingAs($admin);

        Livewire::test(EditTask::class, ['record' => $task->getKey()])
            ->fillForm([
                'subject' => 'tsk.nestorpons.com',
                'title' => $task->title,
                'description' => $task->description,
                'priority' => 'high',
                'status' => 'completed',
                'estimated_hours' => 2,
                'due_date' => '2026-10-01',
            ])
            ->call('save')
            ->assertHasNoFormErrors();

        $this->assertDatabaseHas('tasks', [
            'id' => $task->id,
            'subject' => 'tsk.nestorpons.com',
            'priority' => 'high',
            'status' => 'completed',
        ]);

        $this->assertDatabaseHas('admin_audit_logs', [
            'action' => 'task.updated',
            'subject_id' => $task->id,
            'user' => $admin->email,
        ]);
    }

    public function test_tasks_cannot_be_created_from_the_panel(): void
    {
        $admin = User::factory()->admin()->create();

        $this->actingAs($admin)
            ->get('/admin/tasks/create')
            ->assertNotFound();
    }
}
