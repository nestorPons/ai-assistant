<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Documento spec.md de la tarea (compartido con core-engine).
     */
    public function up(): void
    {
        if (! Schema::hasColumn('tasks', 'spec_md')) {
            Schema::table('tasks', function (Blueprint $table): void {
                $table->mediumText('spec_md')->nullable()->after('specifications');
            });
        }
    }

    public function down(): void
    {
        if (Schema::hasColumn('tasks', 'spec_md')) {
            Schema::table('tasks', function (Blueprint $table): void {
                $table->dropColumn('spec_md');
            });
        }
    }
};
