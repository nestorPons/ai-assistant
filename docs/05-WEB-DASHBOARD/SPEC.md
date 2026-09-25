# SPEC: Web Dashboard (Laravel + Filament)

Estado: **decisiones cerradas, pendiente de implementación**.
Relacionado: [docs/04-DASHBOARD.md](../04-DASHBOARD.md) (TUI actual), [docs/01-INITIAL/SPEC.md](../01-INITIAL/SPEC.md).

## 1. Objetivo
- Exponer por web, en un contenedor PHP, un panel de administración con los **mismos paneles** que el dashboard TUI actual.
- Acceso restringido a un **único usuario `admin`**.
- Al quedar expuesto públicamente, aplicar **medidas de seguridad obligatorias** (§8).

## 2. Alcance
### Incluido
- Nuevo servicio Docker `web-dashboard` (Laravel + Filament).
- Secciones con paridad funcional: Inicio, Tareas, Clientes, Mensajes, Configuración y Logs.
- Autenticación de un solo usuario y cierre de sesión.
- Datos desde la MariaDB compartida del servicio `db` existente.
- Endurecimiento web: TLS (Traefik en producción), cabeceras, rate-limit, sesión segura y auditoría.
- `docker-compose.yml` (desarrollo) y `docker-production.yml` (producción, fuera de git).

### Fuera de alcance
- Sustituir o modificar el TUI Go ni `core-engine`.
- API pública para terceros.
- Multi-usuario, roles o registro público.
- Edición web de `.env` (decisión: **no permitida**, se mantiene en TUI/SSH).
- Doble factor (2FA) e IP allowlist (descartados por decisión del propietario).

## 3. Stack propuesto
- PHP 8.3 + Laravel 12 (última estable verificada al implementar).
- Filament v4 (panel, resources, widgets, acciones).
- MariaDB existente (servicio `db`).
- Nginx + PHP-FPM dentro del contenedor (alternativa: FrankenPHP/Octane).
- Reverse proxy de producción: **Traefik existente** en la red externa `traefik-net`.
- Composer; sin pipeline de assets propio (Filament ya incluye los suyos).

## 4. Arquitectura

```
[ Internet ] --> [ Traefik :443 / TLS ] --> [ web-dashboard :80 ]
                                                   |
                                                   v
                                              [ db (MariaDB) ]  (red interna)
[ core-engine ] -------------------------------------^
[ llm-anonymizer ]  (sin acceso desde web-dashboard)
```

- `web-dashboard` obtiene los datos directamente de la MariaDB del servicio `db`.

### 4.1. Acceso a datos (decisión: Opción A)
- **Eloquent directo** sobre las tablas existentes (`clients`, `raw_messages`, `tasks`, `sync_state`).
- Modelos en `App\Models\Kos`, sin duplicar lógica de negocio; las validaciones de escritura replican las del dominio.
- No se modifica `internal/httpapi` de `core-engine`.

## 5. Paneles (paridad con el TUI)

| Sección | TUI actual | Web (Filament) |
|---|---|---|
| Inicio | contadores clientes (total/autorizados), tareas por estado y prioridad, mensajes (total/sin procesar) | Widgets `StatsOverview` + gráficos |
| Tareas | tabla, filtro por estado, editar `title`, `description`, `subject`, `priority`, `status`, `estimated_hours`, `due_date` | `TaskResource`: tabla + filtros + form + acción cambiar estado |
| Clientes | listar todos/rastreados, alta, editar nombre, alternar `tracked`/`active` | `ClientResource`: CRUD + toggles + filtro `tracked` |
| Mensajes | listado + detalle del contenido original | `MessageResource` (solo lectura) + vista detalle. **Se muestra el contenido original** |
| Configuración | lista de claves `.env`, editar/añadir, ocultar/mostrar secretos | **Excluido** (§9) |
| Logs | actividad del panel (login, ediciones, errores) | `AuditLogResource` + visor de logs (§10) |

Campos y reglas de Tareas/Clientes se replican tal cual (`subject` y `due_date` incluidos).
El contenido de `raw_messages` es sensible: solo accesible tras login y registrado en auditoría al abrir el detalle.

## 6. Modelo de datos
- **Reutiliza** las tablas actuales; sin migraciones destructivas.
- Nueva tabla `admin_audit_logs`:
  `id, user, action, subject_type, subject_id, changes JSON, ip, user_agent, created_at`.
- Tablas auxiliares de Laravel si se usan drivers DB: `sessions`, `cache`, `jobs` (o usar `file`).

## 7. Variables de entorno nuevas
```
WEB_ADMIN_USER=admin
WEB_ADMIN_NAME=admin
WEB_ADMIN_EMAIL=
WEB_ADMIN_PASSWORD_HASH=        # generado por artisan; nunca en claro en producción

APP_URL=https://tsk.nestorpons.com
APP_ENV=production
APP_DEBUG=false
APP_KEY=                        # php artisan key:generate
SESSION_SECURE_COOKIE=true
SESSION_ENCRYPT=true
SESSION_LIFETIME=60
SESSION_IDLE_TIMEOUT=30
TRUSTED_PROXIES=                # Traefik
DB_*                            # mismas credenciales MARIADB_*
```
- `.env` del panel fuera del docroot y con permisos `0600`.

## 8. Seguridad (obligatoria)
- **Autenticación**: usuario único `admin`; registro y reset de contraseña **deshabilitados** en producción.
  Hash Argon2id/bcrypt vía Laravel. Alta mediante seeder/artisan.
- **Fuerza bruta**: rate-limit de login (p. ej. 5/min) y bloqueo progresivo; `fail2ban` en el host.
- **TLS**: gestionado por Traefik (`certresolver`), HTTPS obligatorio y HSTS.
- **Sesión**: cookies `Secure`+`HttpOnly`+`SameSite=Lax`, `SESSION_ENCRYPT=true`,
  regeneración de ID al autenticar, expiración por inactividad.
- **CSRF/XSS**: protecciones por defecto de Laravel/Filament; CSP estricta.
- **Cabeceras**: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy`, `Permissions-Policy`.
- **Aislamiento**: MariaDB y `llm-anonymizer` en red interna, sin puertos publicados.
- **Contenedor**: ejecución como usuario no root, sin `docker.sock`, filesystem de solo lectura
  donde sea posible.
- **Auditoría**: registro de login/logout, ediciones, cambios de estado, apertura de mensajes y errores.
- **Secretos**: nunca mostrar `OPENAI_API_KEY`, `JEV_API_KEY`, tokens ni `APP_KEY` en la web.
- **Endurecer lo existente** (acción recomendada aparte): `core-engine:8081` está publicado sin
  auth y la MariaDB publica `3307:3306`; mover ambos a la red interna.
- `APP_DEBUG=false` y logs sin datos sensibles; rotación y backup.
- **Descartado por decisión**: 2FA y allowlist de IP.

## 9. Configuración (`.env`) — decisión: NO
- No se permite ver ni editar `.env` desde la web.
- La sección "Configuración" se elimina del panel web; se mantiene en el TUI/SSH.

## 10. Logs
- `AuditLogResource` (acciones del panel) + página "Logs" con los logs de Laravel.
- Logs de `core-engine`: opcional vía endpoint interno o volumen; **nunca** montar `docker.sock`.

## 11. Docker / Compose
- `Dockerfile.web-dashboard` multi-stage (composer → assets → php-fpm+nginx), escucha en `:80`.
- **Desarrollo** (`docker-compose.yml`, versionado): servicio `web-dashboard`
  - `depends_on: db (service_healthy)`
  - `networks: internal`
  - publica `8082:80` solo para pruebas locales
  - variables desde `.env`
- **Producción** (`docker-production.yml`, **en `.gitignore`**, subida a mano):
  - `web-dashboard` conectado a la red externa `traefik-net`
  - sin puertos publicados; expuesto por Traefik
  - labels:
    ```
    traefik.enable=true
    traefik.docker.network=traefik-net
    traefik.http.routers.tsk.rule=Host(`tsk.nestorpons.com`)
    traefik.http.routers.tsk.entrypoints=web,websecure
    traefik.http.routers.tsk.tls=true
    traefik.http.routers.tsk.tls.certresolver=myresolver
    traefik.http.services.tsk.loadbalancer.server.port=80
    ```
  - MariaDB: usar la del stack (`db`) o la existente según despliegue; red interna compartida.
- Añadir `docker-production.yml` a `.gitignore`.
- Volumen para `storage/logs`.

## 12. Decisiones cerradas
1. Datos: **Eloquent directo** sobre la MariaDB compartida (Opción A).
2. Edición web de `.env`: **no**.
3. Reverse proxy: **Traefik** de producción (red externa `traefik-net`).
4. 2FA: **no**. IP allowlist: **no**.
5. Contenido original de mensajes: **sí se muestra** (tras login, con auditoría).
6. Dominio: **tsk.nestorpons.com**.
7. Versiones: Laravel 12 + Filament v4 (confirmar al implementar).
8. Compose: `docker-compose.yml` (dev) + `docker-production.yml` (prod, gitignored).

## 13. Entregables
- Proyecto Laravel+Filament (ubicación `web-dashboard/` a acordar).
- `Dockerfile.web-dashboard`, servicio en `docker-compose.yml` y `docker-production.yml` (gitignored).
- Seeder/CLI del usuario admin y generación de hash.
- Entrada en `.gitignore` para `docker-production.yml`.
- Documentación `docs/05-WEB-DASHBOARD/` y tests.

## 14. Criterios de aceptación
- Panel accesible en `https://tsk.nestorpons.com` con login; **sin** acceso anónimo ni registro.
- Checklist de paridad de paneles (§5) verificado contra el TUI (Configuración excluida).
- Datos leídos/escritos en la MariaDB compartida sin afectar a `core-engine`.
- Medidas de §8 activas y verificables (cabeceras, cookies, rate-limit, auditoría).
- Auditoría registra login y cambios de datos.
- Producción levantada con `docker-production.yml` vía Traefik y TLS; dev con `docker-compose.yml`.

## 15. Pruebas
- Feature tests: autenticación, autorización, CRUD, validaciones y auditoría.
- Security checks: rate-limit, cabeceras, cookies, CSRF, redirección HTTPS.
- Manual: checklist de paridad contra el dashboard TUI.

## 16. Plan por fases
1. Scaffold Laravel+Filament + Dockerfile + login admin.
2. Modelos Eloquent + resources Tareas/Clientes/Mensajes (con contenido original).
3. Inicio (widgets) + auditoría/Logs.
4. Endurecimiento: cabeceras, rate-limit, sesión, TLS vía Traefik.
5. `docker-compose.yml` (dev) + `docker-production.yml` (prod, gitignored).
6. Tests + documentación.
