<?php

namespace Tests\Feature\Panel;

use App\Filament\Pages\TaskBoard;
use App\Models\TskHub\Task;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Livewire\Livewire;
use Tests\TestCase;

class TaskBoardTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_can_view_the_jira_board(): void
    {
        $admin = User::factory()->admin()->create();
        Task::factory()->create(['status' => 'pending', 'title' => 'Tablero Kanban']);

        $this->actingAs($admin)
            ->get(TaskBoard::getUrl())
            ->assertSuccessful()
            ->assertSee('Tablero');
    }

    public function test_admin_can_move_a_task_to_another_column(): void
    {
        $admin = User::factory()->admin()->create();
        $task = Task::factory()->create(['status' => 'pending']);
        $this->actingAs($admin);

        Livewire::test(TaskBoard::class)
            ->call('moveTask', $task->getKey(), 'in_progress')
            ->assertHasNoErrors();

        $this->assertDatabaseHas('tasks', [
            'id' => $task->id,
            'status' => 'in_progress',
        ]);

        $this->assertDatabaseHas('admin_audit_logs', [
            'action' => 'task.updated',
            'subject_id' => $task->id,
        ]);
    }

    public function test_board_ignores_invalid_statuses(): void
    {
        $admin = User::factory()->admin()->create();
        $task = Task::factory()->create(['status' => 'pending']);
        $this->actingAs($admin);

        Livewire::test(TaskBoard::class)
            ->call('moveTask', $task->getKey(), 'no_existe');

        $this->assertDatabaseHas('tasks', [
            'id' => $task->id,
            'status' => 'pending',
        ]);
    }
}
