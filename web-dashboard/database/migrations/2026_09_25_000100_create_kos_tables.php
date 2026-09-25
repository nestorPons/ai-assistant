<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/**
 * Espejo de las tablas de core-engine. Se crean solo si no existen para no
 * interferir con las migraciones del motor cuando comparten la misma MariaDB.
 */
return new class extends Migration
{
    public function up(): void
    {
        if (! Schema::hasTable('clients')) {
            Schema::create('clients', function (Blueprint $table): void {
                $table->uuid('id')->primary();
                $table->enum('source', ['whatsapp', 'telegram', 'gmail']);
                $table->string('identifier');
                $table->string('name')->default('');
                $table->boolean('tracked')->default(false);
                $table->boolean('active')->default(true);
                $table->timestamp('created_at')->nullable();
                $table->timestamp('updated_at')->nullable();
                $table->unique(['source', 'identifier'], 'uq_clients_source_identifier');
            });
        }

        if (! Schema::hasTable('raw_messages')) {
            Schema::create('raw_messages', function (Blueprint $table): void {
                $table->uuid('id')->primary();
                $table->uuid('client_id');
                $table->enum('source', ['whatsapp', 'telegram', 'gmail']);
                $table->string('external_id');
                $table->text('content');
                $table->boolean('processed')->default(false);
                $table->timestamp('created_at')->nullable();
                $table->unique(['source', 'external_id'], 'uq_raw_messages_source_external');
                $table->index('client_id', 'idx_raw_messages_client');
                $table->foreign('client_id')->references('id')->on('clients')->cascadeOnDelete();
            });
        }

        if (! Schema::hasTable('tasks')) {
            Schema::create('tasks', function (Blueprint $table): void {
                $table->uuid('id')->primary();
                $table->uuid('client_id');
                $table->uuid('message_id');
                $table->string('title', 500);
                $table->text('description');
                $table->string('subject')->default('');
                $table->enum('priority', ['low', 'medium', 'high'])->default('medium');
                $table->decimal('estimated_hours', 6, 2)->nullable();
                $table->date('due_date')->nullable();
                $table->json('specifications')->nullable();
                $table->enum('status', ['pending', 'in_progress', 'completed', 'discarded', 'needs_review'])->default('pending');
                $table->float('ai_confidence')->default(0);
                $table->timestamp('created_at')->nullable();
                $table->timestamp('updated_at')->nullable();
                $table->index('client_id', 'idx_tasks_client');
                $table->index('message_id', 'idx_tasks_message');
                $table->index('status', 'idx_tasks_status');
                $table->foreign('client_id')->references('id')->on('clients')->cascadeOnDelete();
                $table->foreign('message_id')->references('id')->on('raw_messages')->cascadeOnDelete();
            });
        }
    }

    public function down(): void
    {
        // No se eliminan tablas compartidas con core-engine.
    }
};
