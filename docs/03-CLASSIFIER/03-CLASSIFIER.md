# Clasificador — Cadena de fallback

## Estado actual
- Jev **deshabilitado temporalmente** (`.env`: `JEV_API_KEY` comentado).
- `CLASSIFIER_MODE=llm`, `CLASSIFIER_MODEL=gpt-4o-mini`.
- Motivo: Jev devuelve 429 (saturación) y clasificaciones erróneas en pruebas live.

## Objetivo
- No depender de un único proveedor (Jev se satura: 429).
- Mantener la calidad de Jev como primario y degradar con elegancia.

## Modos (`CLASSIFIER_MODE`)
- `chain` (default): Jev → LLM remoto → reglas → revisión.
- `jev`: solo Jev (fallback a reglas si falta `JEV_API_KEY`).
- `llm`: solo LLM remoto (fallback a reglas si falta `OPENAI_API_KEY`).

## Componentes
- `internal/classifier/jev`: Jev (scoring de opciones, inmune a prompt injection).
- `internal/classifier/remote`: LLM con salida estructurada (`json_schema` estricto,
  `temperature=0`) sobre `llm.LLMProvider` (OpenAI-compatible).
- `internal/classifier/chain.go`: recorre etapas; si todas fallan → `NeedsReview`.
- `classifier.RouteDecision`: vocabulario compartido
  (`proceed_fast|deep_review|split_task|block` → `extract|db_action|discard`).

## Config
- `CLASSIFIER_MODE` = `chain` | `jev` | `llm`.
- `CLASSIFIER_MODEL` (por defecto `OPENAI_MODEL`).
- Reusa `OPENAI_API_KEY` / `OPENAI_BASE_URL` (sirve para OpenAI, Groq, Together,
  Fireworks, OpenRouter, vLLM/Ollama: solo cambia la base URL).

## Seguridad
- El pipeline ofusca antes de clasificar.
- El contenido se envía delimitado en `<mensaje>` y el prompt indica ignorar
  instrucciones embebidas (mitiga prompt injection del LLM generativo).
- Ante parseo inválido o fallo total → `NeedsReview`.

## Cambiar de proveedor
- Groq: `OPENAI_BASE_URL=https://api.groq.com/openai/v1`, `CLASSIFIER_MODEL=openai/gpt-oss-20b`.
- OpenRouter: `OPENAI_BASE_URL=https://openrouter.ai/api/v1`.
- Local: `OPENAI_BASE_URL=http://classifier-local:11434/v1` (Ollama).

## Reactivar Jev
- Descomentar `JEV_API_KEY` en `.env` y usar `CLASSIFIER_MODE=chain` (o `jev`).

## Test live
```bash
CLASSIFIER_LIVE_TEST=1 go test ./internal/classifier/ -run TestLiveClassifiers -v
```
- Clasifica un texto casual (no tarea) y una petición de trabajo, imprime el JSON.
- Si un proveedor da error retryable (p. ej. 429) el subtest se marca como *skip*.
- Resultados observados: LLM correcto; Jev con falso positivo y 429.
