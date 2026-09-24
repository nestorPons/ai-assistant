# Gmail Push (Pub/Sub) — Setup

## Objetivo
- Dejar de releer los mismos emails.
- Sincronización incremental por `historyId` + notificaciones Pub/Sub (pull).
- Sin exponer endpoint público.

## Modos
- `poll`: consulta por query (`is:unread`) cada `poll`. Sin cursor.
- `pull`: `users.watch` + History API + worker pull de Pub/Sub + polling de respaldo.

## Setup en Google Cloud
1. Habilitar APIs: Gmail API y Pub/Sub API.
2. Crear topic `gmail-push`.
3. Dar rol Pub/Sub Publisher a `gmail-api-push@system.gserviceaccount.com` sobre el topic.
4. Crear suscripción **pull** `gmail-push-sub` sobre el topic.
5. Re-autorizar con `cmd/gmail-auth` (scope `gmail.readonly` + `pubsub`); regenera `GMAIL_REFRESH_TOKEN`.

## Variables de entorno
- `GMAIL_CLIENT_ID`, `GMAIL_CLIENT_SECRET`, `GMAIL_REFRESH_TOKEN`
- `GMAIL_PUSH_MODE=pull`
- `GMAIL_PUBSUB_TOPIC=projects/<project>/topics/gmail-push`
- `GMAIL_PUBSUB_SUBSCRIPTION=projects/<project>/subscriptions/gmail-push-sub`
- `GMAIL_POLL_FALLBACK=5m`

## Flujo
1. Primer arranque sin cursor: `getProfile` guarda `historyId` como baseline (no procesa backlog).
2. `users.watch` registra el topic y guarda expiración; se renueva cada 6h.
3. Notificación Pub/Sub → `Sync`: `history.list(startHistoryId=cursor)` → mensajes nuevos → emitir → guardar `historyId`.
4. Historial expirado (404) → resync con `messages.list` y nuevo baseline.
5. Polling de respaldo cada `GMAIL_POLL_FALLBACK` por si falla watch/Pub/Sub.

## Idempotencia
- Entrega Pub/Sub es at-least-once.
- `pipeline.Process` deduplica por `(source, external_id)` antes de crear tareas.
- Cursor serializado con mutex: worker y ticker no lo pisan.

## Persistencia
- Tabla `sync_state` (`source`, `cursor`, `expires_at`, `updated_at`).
- Migración `002_sync_state.sql`.
