<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Administrador único del panel
    |--------------------------------------------------------------------------
    |
    | El panel web solo permite el acceso de un usuario administrador. El alta
    | se realiza con el seeder `AdminUserSeeder`, que crea o actualiza el usuario
    | a partir de estas variables. Nunca se registra la contraseña en logs.
    |
    */

    'admin_name' => env('WEB_ADMIN_NAME', 'admin'),

    'admin_email' => env('WEB_ADMIN_EMAIL', 'admin@example.com'),

    'admin_password' => env('WEB_ADMIN_PASSWORD'),

    /*
    |--------------------------------------------------------------------------
    | Proxy de confianza
    |--------------------------------------------------------------------------
    |
    | En producción el TLS lo termina Traefik. Se confía en los proxies para
    | que Laravel detecte HTTPS vía X-Forwarded-Proto. Ajustar si es necesario.
    |
    */

    'trusted_proxies' => env('TRUSTED_PROXIES', '*'),

    /*
    |--------------------------------------------------------------------------
    | Idle timeout del panel (minutos)
    |--------------------------------------------------------------------------
    */

    'session_idle_timeout' => (int) env('SESSION_IDLE_TIMEOUT', 30),

];
