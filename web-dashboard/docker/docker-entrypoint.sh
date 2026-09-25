#!/bin/sh
set -eu

cd /var/www/html

if [ -z "${APP_KEY:-}" ] && [ ! -f .env ]; then
    echo "AVISO: APP_KEY no configurada. Genera una con 'php artisan key:generate'." >&2
fi

# Assets de Filament (public/css, public/js, public/fonts)
php artisan filament:assets --ansi >/dev/null 2>&1 || true

# Esquema: crea solo las tablas propias y las de K-OS si no existen.
php artisan migrate --force --no-interaction

# Alta/actualización del administrador único si se ha definido la contraseña.
if [ -n "${WEB_ADMIN_PASSWORD:-}" ]; then
    php artisan db:seed --class=AdminUserSeeder --force --no-interaction
fi

php artisan optimize:clear >/dev/null 2>&1 || true
php artisan optimize

exec /usr/bin/supervisord -c /etc/supervisord.conf
