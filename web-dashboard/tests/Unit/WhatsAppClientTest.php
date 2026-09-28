<?php

namespace Tests\Unit;

use App\Services\WhatsAppClient;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class WhatsAppClientTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();

        config(['web.whatsapp_engine_url' => 'http://engine.test']);
    }

    public function test_status_reports_connected(): void
    {
        Http::fake([
            'http://engine.test/whatsapp/status' => Http::response(['connected' => true, 'qr' => '', 'error' => '']),
        ]);

        $status = app(WhatsAppClient::class)->status();

        $this->assertTrue($status['connected']);
        $this->assertSame('', $status['qr']);
    }

    public function test_status_returns_qr_payload_when_pending(): void
    {
        Http::fake([
            'http://engine.test/whatsapp/status' => Http::response(['connected' => false, 'qr' => 'payload', 'error' => '']),
        ]);

        $status = app(WhatsAppClient::class)->status();

        $this->assertFalse($status['connected']);
        $this->assertSame('payload', $status['qr']);
    }

    public function test_status_returns_error_when_engine_unreachable(): void
    {
        Http::fake([
            'http://engine.test/*' => Http::failedConnection(),
        ]);

        $status = app(WhatsAppClient::class)->status();

        $this->assertFalse($status['connected']);
        $this->assertNotEmpty($status['error']);
    }

    public function test_qr_data_uri_is_built_from_png(): void
    {
        Http::fake([
            'http://engine.test/whatsapp/qr.png' => Http::response("\x89PNG-fake", 200, ['Content-Type' => 'image/png']),
        ]);

        $uri = app(WhatsAppClient::class)->qrDataUri();

        $this->assertSame('data:image/png;base64,'.base64_encode("\x89PNG-fake"), $uri);
    }
}
