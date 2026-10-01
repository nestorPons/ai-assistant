<?php

namespace Tests\Unit;

use App\Services\ChannelsClient;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class ChannelsClientTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();

        config(['web.whatsapp_engine_url' => 'http://engine.test']);
    }

    public function test_statuses_returns_all_channels(): void
    {
        Http::fake([
            'http://engine.test/channels/status' => Http::response(['channels' => [
                ['name' => 'gmail', 'enabled' => true, 'connected' => true, 'mode' => 'poll', 'error' => '', 'detail' => ''],
                ['name' => 'telegram', 'enabled' => false, 'connected' => false, 'mode' => '', 'error' => 'no configurado', 'detail' => ''],
                ['name' => 'whatsapp', 'enabled' => true, 'connected' => false, 'mode' => 'qr', 'error' => 'esperando QR', 'detail' => ''],
            ]]),
        ]);

        $statuses = app(ChannelsClient::class)->statuses();

        $this->assertCount(3, $statuses);
        $this->assertSame('gmail', $statuses[0]['name']);
        $this->assertTrue($statuses[0]['connected']);
        $this->assertFalse($statuses[1]['enabled']);
        $this->assertFalse($statuses[2]['connected']);
        $this->assertSame('esperando QR', $statuses[2]['error']);
    }

    public function test_statuses_returns_fallback_when_engine_unreachable(): void
    {
        Http::fake([
            'http://engine.test/*' => Http::failedConnection(),
        ]);

        $statuses = app(ChannelsClient::class)->statuses();

        $this->assertCount(3, $statuses);
        foreach ($statuses as $channel) {
            $this->assertFalse($channel['connected']);
            $this->assertNotEmpty($channel['error']);
        }
    }
}
