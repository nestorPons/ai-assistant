# ai-assistant
K-OS personal assistant

Backend desacoplado en Go: recibe mensajes de Gmail/WhatsApp/Telegram, filtra usuarios autorizados, ofusca PII, clasifica con Jev y extrae tareas con un LLM estructurado.

## Estructura
- `cmd/core-engine` — orquestador principal.
- `cmd/llm-anonymizer` — microservicio de ofuscación de PII.
- `internal/…` — dominio, persistencia, ingestion, pipeline, etc.

## Arranque rápido (demo autocontenida)
```bash
go run ./cmd/core-engine
```
Sin credenciales usa: almacén en memoria, anonimizador embebido, clasificador de reglas y extractor simulado.

## Arranque completo (Docker Compose)
```bash
cp .env.example .env   # rellena JEV_API_KEY / OPENAI_API_KEY / GMAIL_ACCESS_TOKEN
docker compose up --build
```

## Tests
```bash
go test ./...
```

## Documentación
- [docs/01-INITIAL](docs/01-INITIAL) — especificación, plan y decisiones.
- [docs/02-IMPLEMENTATION](docs/02-IMPLEMENTATION) — notas de implementación.
