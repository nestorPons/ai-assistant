<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Se elimina estimated_hours: no es necesario en el flujo de tareas.
     */
    public function up(): void
    {
        if (Schema::hasColumn('tasks', 'estimated_hours')) {
            Schema::table('tasks', function (Blueprint $table): void {
                $table->dropColumn('estimated_hours');
            });
        }
    }

    public function down(): void
    {
        if (! Schema::hasColumn('tasks', 'estimated_hours')) {
            Schema::table('tasks', function (Blueprint $table): void {
                $table->decimal('estimated_hours', 6, 2)->nullable()->after('priority');
            });
        }
    }
};
