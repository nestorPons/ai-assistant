<?php

namespace Tests\Feature\Panel;

use App\Filament\Resources\Clients\Pages\CreateClient;
use App\Models\Kos\Client;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Livewire\Livewire;
use Tests\TestCase;

class ClientResourceTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_can_create_a_client_and_the_action_is_audited(): void
    {
        $admin = User::factory()->admin()->create();
        $this->actingAs($admin);

        Livewire::test(CreateClient::class)
            ->fillForm([
                'source' => 'gmail',
                'identifier' => 'nuevo@example.com',
                'name' => 'Nuevo Cliente',
                'tracked' => true,
                'active' => true,
            ])
            ->call('create')
            ->assertHasNoFormErrors();

        $this->assertDatabaseHas('clients', [
            'identifier' => 'nuevo@example.com',
            'name' => 'Nuevo Cliente',
        ]);

        $this->assertDatabaseHas('admin_audit_logs', [
            'action' => 'client.created',
            'user' => $admin->email,
        ]);
    }

    public function test_duplicate_client_in_the_same_channel_is_rejected(): void
    {
        $admin = User::factory()->admin()->create();
        $this->actingAs($admin);

        Client::factory()->create([
            'source' => 'gmail',
            'identifier' => 'repetido@example.com',
        ]);

        Livewire::test(CreateClient::class)
            ->fillForm([
                'source' => 'gmail',
                'identifier' => 'repetido@example.com',
                'name' => 'Duplicado',
                'tracked' => true,
                'active' => true,
            ])
            ->call('create')
            ->assertHasFormErrors(['identifier']);
    }
}
