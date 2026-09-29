<?php

namespace App\Filament\Pages;

use App\Filament\Resources\Tasks\TaskResource;
use App\Models\TskHub\Task;
use BackedEnum;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Support\Icons\Heroicon;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Database\Eloquent\Collection;

class TaskBoard extends Page
{
    protected static string|BackedEnum|null $navigationIcon = Heroicon::OutlinedViewColumns;

    protected static ?string $navigationLabel = 'Tablero Jira';

    protected static ?int $navigationSort = 5;

    protected string $view = 'filament.pages.task-board';

    public string $search = '';

    public string $priority = '';

    public static function getNavigationLabel(): string
    {
        return 'Tablero Jira';
    }

    public function getTitle(): string|Htmlable
    {
        return 'Tablero de tareas · estilo Jira';
    }

    /**
     * @return array<string, array{label: string, color: string}>
     */
    public function getColumns(): array
    {
        return [
            'pending' => ['label' => 'Pendiente', 'color' => 'warning'],
            'in_progress' => ['label' => 'En progreso', 'color' => 'info'],
            'needs_review' => ['label' => 'Revisión', 'color' => 'gray'],
            'completed' => ['label' => 'Completada', 'color' => 'success'],
            'discarded' => ['label' => 'Descartada', 'color' => 'danger'],
        ];
    }

    /**
     * @return array<string, Collection<int, Task>>
     */
    public function getBoardData(): array
    {
        $board = [];

        foreach (array_keys($this->getColumns()) as $status) {
            $board[$status] = $this->getTasksByStatus($status);
        }

        return $board;
    }

    /**
     * @return Collection<int, Task>
     */
    protected function getTasksByStatus(string $status): Collection
    {
        $query = Task::query()
            ->with('client')
            ->where('status', $status)
            ->when(filled($this->priority), fn ($q) => $q->where('priority', $this->priority))
            ->when(filled($this->search), function ($q): void {
                $term = '%'.str_replace(['%', '_'], '', $this->search).'%';
                $q->where(function ($inner) use ($term): void {
                    $inner->where('title', 'like', $term)
                        ->orWhere('subject', 'like', $term)
                        ->orWhere('description', 'like', $term)
                        ->orWhereHas('client', fn ($c) => $c->where('name', 'like', $term));
                });
            })
            ->orderByRaw("CASE WHEN priority='high' THEN 0 WHEN priority='medium' THEN 1 ELSE 2 END")
            ->orderByDesc('created_at')
            ->limit(50);

        return $query->get();
    }

    public function moveTask(string $taskId, string $status): void
    {
        if (! in_array($status, Task::STATUSES, true)) {
            return;
        }

        $task = Task::query()->find($taskId);

        if (! $task) {
            return;
        }

        if ($task->status === $status) {
            return;
        }

        $task->update(['status' => $status]);

        Notification::make()
            ->success()
            ->title('Tarea movida a '.$this->getColumns()[$status]['label'])
            ->send();
    }

    public function editUrl(Task $task): string
    {
        return TaskResource::getUrl('edit', ['record' => $task]);
    }
}
