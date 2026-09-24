# Implementación (MVP)

Estado: MVP funcional y testeado. `SPEC.md`/`PLAN.md`/`DECISIONS.md` en `docs/01-INITIAL`.

## Lo construido

### `core-engine`
- **Config** (`internal/config`): variables de entorno, sin secretos en logs.
- **Dominio** (`internal/domain`): `IncomingMessage`, `Client`, `RawMessage`, `Task`, `Specifications`.
- **Persistencia** (`internal/persistence`): migraciones SQL (`clients`, `raw_messages`, `tasks`, `sync_state`), `MySQLStore` y `MemoryStore` (pruebas/demo). Índices de unicidad `clients(source,identifier)` y `raw_messages(source,external_id)`.
- **Ingestión** (`internal/ingestion`): interfaz `Source`; fuentes `simulated`, `gmail` (polling API REST e incremental por History API + Pub/Sub pull) y `pubsub` (worker pull).
- **Normalizador** (`internal/normalizer`): valida canal, recorta y normaliza identificador.
- **Anonimizador cliente** (`internal/anonymizer`): `Client` HTTP hacia `llm-anonymizer`.
- **Contexto** (`internal/context`): `Builder` genera sobre JSON con mensaje ofuscado delimitado.
- **Clasificador** (`internal/classifier`): interfaz `Classifier`; `jev.Provider` (Jev), `remote.Classifier` (LLM con salida estructurada) y `MockRuleClassifier`; `Chain` de fallback; `RouteDecision` (vocabulario compartido).
- **Extractor** (`internal/extractor`): interfaz `Extractor`; `LLMExtractor` (Structured Outputs vía `LLMProvider`) y `MockExtractor`.
- **LLM** (`internal/llm`): `LLMProvider`, `CompletionRequest/Response`, errores normalizados; adaptador `openai.Provider` (OpenAI-compatible). `internal/oauth` provee el `TokenSource` de Google.
- **Pipeline** (`internal/pipeline`): orden obligatorio (autorizar → idempotencia → guardar → pre-filtro → ofuscar → contexto → clasificador → ruta).
- **Worker** (`internal/worker`): goroutines con límite de concurrencia y reintentos exponenciales.
- **API mínima** (`internal/httpapi`): gestión de `clients` y `tasks`.

### `llm-anonymizer`
- Contratos `SanitizeRequest/Response`, `RevertRequest/Response`, `MappingTable`, `MappingStore` + `MemoryMappingStore` (TTL 5 min).
- Detección heurística (`internal/anonymizer/detect`): email, teléfono, DNI/NIE (módulo 23), IBAN (mod 97), tarjetas (Luhn), diccionarios Aho-Corasick.
- Motor (`internal/anonymizer`): resolución de solapamientos + tokens `{{TIPO_N}}` estables.
- `POST /api/sanitize` y `POST /api/revert` con autenticación Bearer, límites de tamaño y errores no sensibles.

## Decisiones de implementación (no especificadas)

1. **Vocabulario de Jev**: `jev.Provider` acepta ambos vocabularios — gating (`proceed_fast|deep_review|split_task|block`) e interno (`db_action|extract|discard`). `proceed_fast` resuelve la ruta vía campo `route` opcional (por defecto `extract`).
2. **NER contextual**: se deja interfaz `detect.NER` con `NoopNER` (no se integra ONNX aún por ausencia de artefactos `model.onnx`/`tokenizer.json`). Los patrones deterministas ya cubren emails, teléfonos, DNI, IBAN y tarjetas.
3. **`db_action`**: soporta solo `create`/`update` de `client` y `update` de `task` vía repositorios tipados; nunca SQL generado por el modelo.
4. **Revisión**: `deep_review`/`split_task`/`block` se registran en log y el mensaje queda procesado (sin acciones automáticas). Baja confianza del extractor → tarea `needs_review`.
5. **Demo autocontenida**: sin `DATABASE_DSN` se usan almacén en memoria, anonimizador embebido y proveedores mock.
6. **Clasificador desacoplado**: `CLASSIFIER_MODE` = `chain` (Jev → LLM → reglas → revisión) | `jev` | `llm`. El clasificador LLM usa salida estructurada (`json_schema` estricto, temperatura 0) sobre `LLMProvider` (OpenAI-compatible).
7. **Jev deshabilitado (temporal)**: por errores 429 (saturación) y clasificaciones erróneas en pruebas live, se usa `CLASSIFIER_MODE=llm` con `gpt-4o-mini`. El adaptador Jev queda en el código y se reactiva con `JEV_API_KEY` + modo `chain`/`jev`.
8. **Gmail incremental**: cursor `historyId` en `sync_state`; `users.watch` + worker Pub/Sub pull + polling de respaldo (`GMAIL_POLL_FALLBACK`).

## Comandos
```bash
go test ./...
go run ./cmd/gmail-auth     # obtener/renovar el refresh token OAuth de Gmail
go run ./cmd/gmail-sync      # sincronización Gmail a demanda (dev)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /llm-anonymizer ./cmd/llm-anonymizer
```

## Tests live (gateados por variable de entorno)
```bash
JEV_LIVE_TEST=1 CLASSIFIER_LIVE_TEST=1 go test ./internal/classifier/ -run TestLiveClassifiers -v
PUBSUB_LIVE_TEST=1 go test ./internal/ingestion/pubsub/ -run TestLivePubSubPull -v
GMAIL_LIVE_TEST=1 go test ./internal/ingestion/gmail/ -run TestLiveGmailLastMessage -v
```
