<?php

namespace App\Services;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;

/**
 * Cliente del core-engine (Go) para el estado agregado de los canales de
 * ingesta (Gmail, Telegram, WhatsApp). Ver GET /channels/status.
 */
class ChannelsClient
{
    /**
     * @return array<int, array{name: string, enabled: bool, connected: bool, mode: string, error: string, detail: string}>
     */
    public function statuses(): array
    {
        try {
            $response = Http::timeout(10)->get($this->baseUrl().'/channels/status');
        } catch (ConnectionException) {
            return $this->fallback('No se puede contactar con el motor');
        }

        if ($response->failed()) {
            return $this->fallback('Motor no disponible');
        }

        $channels = $response->json('channels');

        if (! is_array($channels)) {
            return $this->fallback('Respuesta inválida del motor');
        }

        return array_values(array_map(fn ($c): array => [
            'name' => (string) ($c['name'] ?? ''),
            'enabled' => (bool) ($c['enabled'] ?? false),
            'connected' => (bool) ($c['connected'] ?? false),
            'mode' => (string) ($c['mode'] ?? ''),
            'error' => (string) ($c['error'] ?? ''),
            'detail' => (string) ($c['detail'] ?? ''),
        ], $channels));
    }

    /**
     * @return array<int, array{name: string, enabled: bool, connected: bool, mode: string, error: string, detail: string}>
     */
    private function fallback(string $error): array
    {
        return array_map(fn (string $name): array => [
            'name' => $name,
            'enabled' => false,
            'connected' => false,
            'mode' => '',
            'error' => $error,
            'detail' => '',
        ], ['gmail', 'telegram', 'whatsapp']);
    }

    private function baseUrl(): string
    {
        return rtrim((string) config('web.whatsapp_engine_url'), '/');
    }
}
