<?php

namespace Tests\Feature\Panel;

use App\Models\User;
use Filament\Facades\Filament;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class PanelConfigTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_panel_has_spa_mode_enabled(): void
    {
        $panel = Filament::getPanel('admin');

        $this->assertTrue($panel->hasSpaMode(), 'El panel admin debe navegar sin recarga completa (modo SPA).');
    }

    public function test_sidebar_shows_the_app_version(): void
    {
        $admin = User::factory()->admin()->create();

        $this->actingAs($admin)
            ->get('/admin')
            ->assertSuccessful()
            ->assertSee('v'.config('app.version'));
    }
}
