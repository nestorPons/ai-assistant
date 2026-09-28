# Dashboard TUI — Panel de administración por SSH

## Objetivo
- Administrar (clientes, tareas, mensajes, `.env`, logs) desde terminal.
- Cero exposición: no abre puertos ni servicios; se ejecuta dentro de una sesión SSH ya autenticada.
- UX estilo Filament (menú lateral + tablas + formularios) con soporte de ratón.

## Stack
- `github.com/rivo/tview` (mismo widget set que `lazysql`).
- Ratón activo (`EnableMouse`); teclado: `↑↓/Tab`, `Enter`, `Esc` (cerrar modal), `Ctrl+C` (salir).

## Arquitectura
- Binario independiente `cmd/dashboard`; reutiliza `persistence.Store` y el `.env`.
- Persistencia: MariaDB si hay `DATABASE_DSN`; si no, almacén en memoria (demo).
- Paquete `internal/dashboard` (UI + lógica de panel).

## Componentes
- `cmd/dashboard/main.go`: flags y wiring (store, auth, DSN).
- `internal/dashboard/app.go`: login, shell, menú lateral, modales, navegación.
- `internal/dashboard/auth.go`: login de un único usuario (PBKDF2-SHA256).
- `internal/dashboard/envfile.go`: editor de `.env` preservando comentarios y orden.
- `internal/dashboard/activity.go`: buffer circular + handler `slog` para logs.
- `internal/dashboard/home.go`: resumen (clientes, tareas, mensajes).
- `internal/dashboard/clients.go`: CRUD de la lista blanca.
- `internal/dashboard/tasks.go`: tabla, filtro por estado, edición y cambio de estado.
- `internal/dashboard/messages.go`: mensajes ingeridos + detalle.
- `internal/dashboard/config.go`: edición de variables `.env` (secretos enmascarados).
- `internal/dashboard/logs.go`: logs de actividad en vivo.
- `internal/dashboard/util.go`: helpers de celdas/colores.

## Secciones
- Inicio: contadores por estado/prioridad y mensajes sin procesar.
- Tareas: filtrar por estado, editar (título, descripción, prioridad, estado, horas).
- Clientes: alta/edición, activar/desactivar, rastrear, ver todos/rastreados.
- Mensajes: listado y detalle del contenido original (acceso interno).
- Configuración: lista de claves del `.env`, editar/añadir, mostrar/ocultar secretos.
- Logs: actividad del panel (login, ediciones, errores).

## Acceso
- Un único usuario: `DASHBOARD_USER` (default `admin`).
- Prefiere `DASHBOARD_PASSWORD_HASH` (PBKDF2-SHA256); `DASHBOARD_PASSWORD` solo desarrollo.
- Sin credenciales configuradas: acceso abierto con aviso (solo desarrollo).

## Ejecución
```bash
# 1) Genera el hash y cópialo al .env
go run ./cmd/dashboard -hash

# 2) Ejecuta dentro de la sesión SSH
go run ./cmd/dashboard
# o compilado:
go build -o dashboard ./cmd/dashboard && ./dashboard
```

## Flags
- `-env <ruta>`: archivo `.env` editable (default `.env`).
- `-dsn <dsn>`: DSN de MariaDB (default `DATABASE_DSN`).
- `-user <usuario>`: usuario del panel (default `DASHBOARD_USER`).
- `-db-host` / `-db-port`: host/puerto al construir el DSN desde `MARIADB_*` (default `127.0.0.1:3307`).
- `-hash`: pide contraseña y muestra su hash, luego sale.

## Resolución del DSN
- Orden: `-dsn` → `DATABASE_DSN` (entorno) → construido desde `MARIADB_*` del `.env`.
- Sin DSN: memoria (los datos no persisten entre ejecuciones).

## Seguridad
- No escucha en red; requiere acceso previo por SSH.
- `.env` se escribe con permisos `0600`.
- Secretos enmascarados por defecto; se revelan solo bajo acción explícita.
- Las credenciales del panel no se registran en logs.

## Tests
```bash
go test ./internal/dashboard/ -v
```
- Cubre hash/verificación, parser de `.env` y un smoke test de todas las vistas.
