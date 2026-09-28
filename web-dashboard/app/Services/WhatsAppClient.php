<?php

namespace App\Services;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;

/**
 * Cliente del core-engine (Go) para consultar el estado de los canales de
 * ingesta. El motor solo es alcanzable desde la red interna de Docker.
 */
class WhatsAppClient
{
    public function status(): array
    {
        $body = $this->get('/whatsapp/status');

        if ($body === null) {
            return ['connected' => false, 'error' => 'No se puede contactar con el motor'];
        }

        return [
            'connected' => (bool) ($body['connected'] ?? false),
            'qr' => (string) ($body['qr'] ?? ''),
            'error' => (string) ($body['error'] ?? ''),
        ];
    }

    public function qrDataUri(): ?string
    {
        $png = $this->get('/whatsapp/qr.png');

        if ($png === null) {
            return null;
        }

        return 'data:image/png;base64,'.base64_encode($png);
    }

    /**
     * @return array<string, mixed>|string|null
     */
    private function get(string $path): array|string|null
    {
        try {
            $response = Http::timeout(5)->get($this->baseUrl().$path);
        } catch (ConnectionException) {
            return null;
        }

        if ($response->failed()) {
            return null;
        }

        $contentType = $response->header('Content-Type') ?? '';

        if (str_contains($contentType, 'image/png')) {
            return $response->body();
        }

        return $response->json();
    }

    private function baseUrl(): string
    {
        return rtrim((string) config('web.whatsapp_engine_url'), '/');
    }
}
