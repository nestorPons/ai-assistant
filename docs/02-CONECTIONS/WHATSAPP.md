# WhatsApp (whatsmeow)

Ingesta de mensajes de WhatsApp en el mismo pipeline que Gmail, mediante
[whatsmeow](https://github.com/tulir/whatsmeow) (WhatsApp Web multi-dispositivo).

## Resumen
- Chats **1:1 y grupos** → `IncomingMessage` (`source=whatsapp`) → filtro de lista blanca → ofuscación → clasificación → extracción de tareas.
- Sesión criptográfica en **SQLite** (archivo/volumen); negocio (clientes, mensajes, tareas) en **MariaDB**.
- Emparejado por **QR**, expuesto en el panel web (Filament).

## Configuración
| Variable | Descripción | Default |
|---|---|---|
| `WHATSAPP_ENABLED` | Activa el listener | `false` |
| `WHATSAPP_DB_PATH` | Archivo SQLite de la sesión | `whatsapp.db` (Docker: `/data/whatsapp.db`) |
| `WHATSAPP_ENGINE_URL` | URL interna del core-engine para el panel | `http://core-engine:8081` |

## Flujo de arranque
1. `Source.Start` abre SQLite (driver puro `modernc.org/sqlite`, sin CGO) y crea el esquema de whatsmeow (`sqlstore`).
2. Si hay sesión guardada → conecta directamente.
3. Si no → emite QR (`GetQRChannel`) y queda a la espera del escaneo; reintenta ante timeouts.
4. Eventos `Message` → `IncomingMessage` → canal del worker.

## Emparejado (QR)
1. `WHATSAPP_ENABLED=true` + arranque.
2. Panel web → Inicio → widget **WhatsApp**: muestra el QR.
3. Teléfono: WhatsApp → Ajustes → **Dispositivos vinculados** → Vincular dispositivo → escanear.
4. Al emparejar, el widget pasa a **Conectado**; la sesión persiste entre reinicios.

## Identificadores (lista blanca `clients`)
| Tipo | `identifier` | Ejemplo |
|---|---|---|
| Chat 1:1 | teléfono `+<dígitos>` | `+34612345678` |
| Grupo | JID `<id>@g.us` | `120363012345678@g.us` |

Alta igual que Gmail: `source=whatsapp`, `tracked=1`, `active=1`.

## Mensajes procesados
- Texto (`conversation`/`extendedText`) y pie de foto (caption) de imagen/vídeo/documento.
- Ignorados: propios (`IsFromMe`), estados/difusiones y mensajes sin texto.

## Endpoints HTTP (core-engine)
- `GET /whatsapp/status` → `{connected, qr?, error?}`.
- `GET /whatsapp/qr.png` → PNG del QR pendiente (para el panel).

## Estado en el panel web
- Widget Filament (`WhatsAppConnectionWidget`) con polling 5 s.
- Conectado → badge verde; sin vincular → QR; error → mensaje.

## Persistencia (reparto)
- **SQLite** (`WHATSAPP_DB_PATH`): dispositivo, claves de identidad, sesiones Signal, pre-keys, sender-keys y estado de la app. Es lo que evita re-escanear el QR en cada reinicio.
- **MariaDB**: `clients`, `raw_messages`, `tasks` (como Gmail).

## Notas / límites
- whatsmeow es **no oficial**; la vinculación puede revocarse desde el teléfono.
- Si se cierra sesión (`LoggedOut`), se marca desconectado; reinicia `core-engine` para un QR nuevo.
- El QR rota cada ~20 s; el panel refresca cada 5 s.
