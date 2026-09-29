# ai-assistant — instrucciones del agente

## Estructura
- `cmd/`, `internal/` — backend Go (core-engine, ingesta, clasificador).
- `web-dashboard/` — panel Laravel 13 + Filament 4 (`app/Filament`, `config/`, `resources/views/filament`).
- `docker-compose.yml` — dev; `docker-production.yml` (gitignored) — prod tras Traefik (`traefik-net`).

## Versionado del panel (obligatorio)
- Fuente única: `web-dashboard/config/app.php` → clave `version` (visible en el sidebar del panel).
- Toda tarea finalizada que cambie código → subir versión en el mismo cambio:
  - `patch` (x.y.Z): fix, estilo, config, tests.
  - `minor` (x.Y.0): feature, página/recurso/widget nuevo.
- Incluir el bump en la lista de archivos modificados del resumen final.
