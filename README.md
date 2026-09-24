# ai-assistant
K-OS personal assistant

Backend desacoplado en Go: recibe mensajes de Gmail/WhatsApp/Telegram, filtra usuarios autorizados, ofusca PII, clasifica (cadena con fallback) y extrae tareas con un LLM estructurado.

## Estructura
- `cmd/core-engine` — orquestador principal.
- `cmd/llm-anonymizer` — microservicio de ofuscación de PII.
- `cmd/gmail-auth` — obtiene el `refresh_token` OAuth de Gmail y lo guarda en `.env`.
- `cmd/gmail-sync` — sincronización Gmail a demanda (dev), con pull de Pub/Sub opcional.
- `internal/…` — dominio, persistencia, ingestion, classifier, pipeline, etc.

## Clasificación
- `CLASSIFIER_MODE`: `chain` (Jev → LLM → reglas → revisión) | `jev` | `llm`.
- El clasificador LLM usa `OPENAI_*` (OpenAI-compatible: Groq, OpenRouter, Ollama…).
- Estado actual: Jev deshabilitado por fallos/saturación; `CLASSIFIER_MODE=llm` con `gpt-4o-mini`.
- Detalle en [docs/03-CLASSIFIER.md](docs/03-CLASSIFIER.md).

## Arranque rápido (demo autocontenida)
```bash
go run ./cmd/core-engine
```
Sin credenciales usa: almacén en memoria, anonimizador embebido, clasificador de reglas y extractor simulado.

## Arranque completo (Docker Compose)
```bash
cp .env.example .env   # rellena OPENAI_API_KEY y/o JEV_API_KEY, y credenciales Gmail
docker compose up --build
```

## Tests
```bash
go test ./...
```

## Documentación
- [docs/01-INITIAL](docs/01-INITIAL) — especificación, plan y decisiones.
- [docs/02-CONECTIONS/GMAIL-PUSH.md](docs/02-CONECTIONS/GMAIL-PUSH.md) — Gmail push (Pub/Sub).
- [docs/03-CLASSIFIER.md](docs/03-CLASSIFIER.md) — clasificador y cadena de fallback.
