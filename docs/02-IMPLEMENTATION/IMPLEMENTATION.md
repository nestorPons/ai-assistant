# Implementación (MVP)

Estado: MVP funcional y testeado. `SPEC.md`/`PLAN.md`/`DECISIONS.md` en `docs/01-INITIAL`.

## Lo construido

### `core-engine`
- **Config** (`internal/config`): variables de entorno, sin secretos en logs.
- **Dominio** (`internal/domain`): `IncomingMessage`, `Client`, `RawMessage`, `Task`, `Specifications`.
- **Persistencia** (`internal/persistence`): migración SQL (`clients`, `raw_messages`, `tasks`), `MySQLStore` y `MemoryStore` (pruebas/demo). Índices de unicidad `clients(source,identifier)` y `raw_messages(source,external_id)`.
- **Ingestión** (`internal/ingestion`): interfaz `Source`; fuentes `simulated` y `gmail` (polling API REST).
- **Normalizador** (`internal/normalizer`): valida canal, recorta y normaliza identificador.
- **Anonimizador cliente** (`internal/anonymizer`): `Client` HTTP hacia `llm-anonymizer`.
- **Contexto** (`internal/context`): `Builder` genera sobre JSON con mensaje ofuscado delimitado.
- **Clasificador** (`internal/classifier`): interfaz `Classifier`; `jev.Provider` (adaptador Jev) y `MockRuleClassifier`.
- **Extractor** (`internal/extractor`): interfaz `Extractor`; `LLMExtractor` (Structured Outputs vía `LLMProvider`) y `MockExtractor`.
- **LLM** (`internal/llm`): `LLMProvider`, `CompletionRequest/Response`, errores normalizados; adaptador `openai.Provider`.
- **Pipeline** (`internal/pipeline`): orden obligatorio (autorizar → idempotencia → guardar → pre-filtro → ofuscar → contexto → Jev → ruta).
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

## Comandos
```bash
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /llm-anonymizer ./cmd/llm-anonymizer
```
