<?php

namespace Tests\Feature\Panel;

use Filament\Facades\Filament;
use Tests\TestCase;

class PanelConfigTest extends TestCase
{
    public function test_admin_panel_has_spa_mode_enabled(): void
    {
        $panel = Filament::getPanel('admin');

        $this->assertTrue($panel->hasSpaMode(), 'El panel admin debe navegar sin recarga completa (modo SPA).');
    }
}
