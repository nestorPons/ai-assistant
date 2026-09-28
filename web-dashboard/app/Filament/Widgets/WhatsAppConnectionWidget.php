<?php

namespace App\Filament\Widgets;

use App\Services\WhatsAppClient;
use Filament\Widgets\Concerns\CanPoll;
use Filament\Widgets\Widget;
use Illuminate\Support\Facades\App;

/**
 * Estado de la conexión con WhatsApp: muestra "Conectado" o el código QR de
 * emparejado cuando el canal no está vinculado.
 */
class WhatsAppConnectionWidget extends Widget
{
    use CanPoll;

    protected static ?int $sort = 10;

    protected int|string|array $columnSpan = 'full';

    /**
     * @var view-string
     */
    protected string $view = 'filament.widgets.whatsapp-connection';

    protected ?string $pollingInterval = '5s';

    /**
     * @return array<string, mixed>
     */
    protected function getViewData(): array
    {
        $client = App::make(WhatsAppClient::class);

        $status = $client->status();

        $qrDataUri = null;
        if (! $status['connected'] && $status['qr'] !== '') {
            $qrDataUri = $client->qrDataUri();
        }

        return [
            'status' => $status,
            'qrDataUri' => $qrDataUri,
        ];
    }
}
