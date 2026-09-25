<?php

namespace App\Filament\Widgets;

use App\Models\Kos\Client;
use App\Models\Kos\RawMessage;
use App\Models\Kos\Task;
use Filament\Support\Icons\Heroicon;
use Filament\Widgets\StatsOverviewWidget;
use Filament\Widgets\StatsOverviewWidget\Stat;

/**
 * Resumen de Inicio con la misma información que el dashboard TUI.
 */
class KosStatsOverview extends StatsOverviewWidget
{
    protected static ?int $sort = 0;

    protected function getStats(): array
    {
        $clientsAuthorized = Client::query()
            ->where('tracked', true)
            ->where('active', true)
            ->count();

        $messagesPending = RawMessage::query()
            ->where('processed', false)
            ->count();

        return [
            Stat::make('Clientes', Client::query()->count())
                ->description($clientsAuthorized.' autorizados')
                ->descriptionIcon(Heroicon::OutlinedUsers)
                ->color('primary'),
            Stat::make('Tareas', Task::query()->count())
                ->description(Task::query()->where('priority', 'high')->count().' de prioridad alta')
                ->descriptionIcon(Heroicon::OutlinedClipboardDocumentList)
                ->color('primary'),
            Stat::make('Pendientes', Task::query()->where('status', 'pending')->count())
                ->descriptionIcon(Heroicon::OutlinedClock)
                ->color('warning'),
            Stat::make('En progreso', Task::query()->where('status', 'in_progress')->count())
                ->descriptionIcon(Heroicon::OutlinedPlayCircle)
                ->color('info'),
            Stat::make('Revisión', Task::query()->where('status', 'needs_review')->count())
                ->descriptionIcon(Heroicon::OutlinedExclamationTriangle)
                ->color('danger'),
            Stat::make('Completadas', Task::query()->where('status', 'completed')->count())
                ->descriptionIcon(Heroicon::OutlinedCheckCircle)
                ->color('success'),
            Stat::make('Mensajes', RawMessage::query()->count())
                ->description($messagesPending.' sin procesar')
                ->descriptionIcon(Heroicon::OutlinedEnvelope)
                ->color('primary'),
        ];
    }
}
