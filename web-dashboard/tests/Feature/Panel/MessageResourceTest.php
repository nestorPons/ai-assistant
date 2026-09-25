<?php

namespace Tests\Feature\Panel;

use App\Filament\Resources\Messages\MessageResource;
use App\Models\Kos\RawMessage;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class MessageResourceTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_sees_the_original_message_content_and_the_view_is_audited(): void
    {
        $admin = User::factory()->admin()->create();
        $message = RawMessage::factory()->create([
            'content' => 'Contenido original del mensaje',
        ]);
        $this->actingAs($admin);

        $this->get(MessageResource::getUrl('view', ['record' => $message->getKey()]))
            ->assertSuccessful()
            ->assertSee('Contenido original del mensaje');

        $this->assertDatabaseHas('admin_audit_logs', [
            'action' => 'message.viewed',
            'subject_id' => $message->getKey(),
            'user' => $admin->email,
        ]);
    }

    public function test_messages_cannot_be_edited_from_the_panel(): void
    {
        $admin = User::factory()->admin()->create();
        $message = RawMessage::factory()->create();
        $this->actingAs($admin);

        $this->get('/admin/messages/'.$message->getKey().'/edit')->assertNotFound();
    }
}
