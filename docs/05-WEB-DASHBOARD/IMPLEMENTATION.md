# Implementación — Web Dashboard (Filament)

Estado: implementado. Spec en [SPEC.md](SPEC.md).

## Stack
- Laravel 13 + Filament 4.13 (PHP 8.4 en contenedor).
- Imagen `Dockerfile.web-dashboard`: `php:8.4-fpm-alpine` + nginx + supervisor.
- Datos: MariaDB del servicio `db` (Eloquent directo, sin tocar `core-engine`).

## Componentes
- **Modelos** (`app/Models/Kos`): `Client`, `RawMessage`, `Task` mapeados a las tablas existentes; `AdminAuditLog`.
- **Migraciones propias**: `admin_audit_logs` e `is_admin` en `users`; las tablas K-OS se crean solo si no existen.
- **Recursos Filament** (`app/Filament/Resources`):
  - `Clients`: CRUD, toggles `tracked`/`active`, filtros; canal/identificador inmutables en edición.
  - `Tasks`: listar/ver/editar (tema, título, descripción, prioridad, estado, horas, fecha límite); sin alta ni borrado.
  - `Messages`: solo lectura; muestra el contenido original; abre el mensaje queda auditado.
  - `AdminAuditLogs` (Logs): solo lectura.
- **Inicio**: widget `KosStatsOverview` con los mismos contadores que el TUI.
- **Auditoría** (`app/Services/AuditLogger`): observadores de `Client`/`Task` + listeners de login/logout.
- **Seguridad**: `SecurityHeaders` (CSP, X-Frame-Options, HSTS en HTTPS), rate-limit de login (Filament), sesión cifrada, `canAccessPanel` por `is_admin`.
- **Admin único**: `AdminUserSeeder` desde `WEB_ADMIN_*`.

## Arranque (desarrollo)
Requisitos en el `.env` raíz: `WEB_APP_KEY`, `WEB_ADMIN_PASSWORD`.
```bash
# Generar APP_KEY para Laravel
cd web-dashboard && php artisan key:generate --show   # copiar a WEB_APP_KEY
cd ..
docker compose up --build
# Panel: http://localhost:8082/admin
```

## Arranque (producción)
`docker-production.yml` está en `.gitignore` (subir a mano). Traefik ya existente en `traefik-net`.
```bash
docker compose -f docker-production.yml up -d --build
# Panel: https://tsk.nestorpons.com/admin
```

## Variables nuevas (`.env`)
- `WEB_APP_KEY`: `base64:...` de Laravel.
- `WEB_APP_URL`: por defecto `https://tsk.nestorpons.com`.
- `WEB_ADMIN_NAME`, `WEB_ADMIN_EMAIL`, `WEB_ADMIN_PASSWORD`.
- En el contenedor: `SESSION_SECURE_COOKIE`, `SESSION_ENCRYPT`, `TRUSTED_PROXIES`, `DB_*`.

## Tests
```bash
cd web-dashboard
php artisan test --compact tests/Feature/Panel
```
Cubre acceso (login/roles), creación de cliente, edición de tarea, vista de mensaje con auditoría, ausencia de alta/edición de mensajes, y cabeceras de seguridad.

## Comandos útiles
```bash
cd web-dashboard
vendor/bin/pint                     # estilo
php artisan test --compact          # suite
php artisan filament:assets         # republicar assets del panel
php artisan db:seed --class=AdminUserSeeder --force
```

## Notas
- Edición de `.env` **excluida** de la web (solo TUI/SSH).
- 2FA e IP allowlist descartados por decisión.
- `core-engine:8081` y MariaDB (`3307`) siguen publicados en `docker-compose.yml` de desarrollo; en producción no se publican.
