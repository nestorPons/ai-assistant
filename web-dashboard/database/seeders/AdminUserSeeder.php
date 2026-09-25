<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;

/**
 * Crea o actualiza el único administrador del panel a partir de WEB_ADMIN_*.
 * La contraseña llega en claro y se hashea con el cast `hashed` del modelo.
 */
class AdminUserSeeder extends Seeder
{
    public function run(): void
    {
        $email = (string) config('web.admin_email');
        $password = (string) config('web.admin_password');

        if ($password === '') {
            $this->command?->warn('WEB_ADMIN_PASSWORD vacío: no se crea ni actualiza el administrador.');

            return;
        }

        $user = User::updateOrCreate(
            ['email' => $email],
            [
                'name' => (string) config('web.admin_name'),
                'password' => $password,
                'is_admin' => true,
                'email_verified_at' => now(),
            ],
        );

        $this->command?->info("Administrador listo: {$user->email}");
    }
}
