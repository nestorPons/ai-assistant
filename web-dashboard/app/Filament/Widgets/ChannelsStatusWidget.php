<?php

namespace App\Filament\Widgets;

use App\Services\ChannelsClient;
use Filament\Widgets\Concerns\CanPoll;
use Filament\Widgets\Widget;
use Illuminate\Support\Facades\App;

/**
 * Estado de la conexión con los canales de comunicación (Gmail, Telegram,
 * WhatsApp) al arrancar el panel, con el mismo estilo que el widget de WhatsApp.
 */
class ChannelsStatusWidget extends Widget
{
    use CanPoll;

    protected static ?int $sort = 5;

    protected int|string|array $columnSpan = 'full';

    /**
     * @var view-string
     */
    protected string $view = 'filament.widgets.channels-status';

    protected function getPollingInterval(): ?string
    {
        return '10s';
    }

    /**
     * @return array<string, mixed>
     */
    protected function getViewData(): array
    {
        $channels = App::make(ChannelsClient::class)->statuses();

        return [
            'channels' => $channels,
        ];
    }
}
