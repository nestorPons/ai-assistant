---
tipo-documento: decisiones-arquitectonicas
estado: MVP
---

# Decisiones arquitectónicas

Este documento es la fuente única de verdad para las decisiones compartidas por `SPEC.md` y `PLAN.md`.

## Atributos fijados

```yaml
lenguaje-backend: Go
base-de-datos: MariaDB
orquestador: Jev
clasificador: cadena-con-fallback
clasificador-modo-env: CLASSIFIER_MODE
clasificador-MVP: LLM-remoto-OpenAI
clasificador-MVP-modelo: gpt-4o-mini
clasificador-estado: Jev-deshabilitado-temporalmente
modelo-NER: Davide/xlm-roberta-base-finetuned-panx-ner
modelo-NER-alternativo: mrm8488/bert-spanish-cased-finetuned-ner
artefacto-modelo-NER: model.onnx
artefacto-tokenizador-NER: tokenizer.json
etiquetas-NER-minimas:
  - PER
  - ORG
  - LOC
servicio-anonimizador: llm-anonymizer
almacenamiento-mapping-MVP: RAM
TTL-mapping: 5m
tabla-contactos-y-autorizacion: clients
campo-especificaciones-tarea: tasks.specifications
tipo-especificaciones-tarea: JSON
interfaz-proveedor-LLM: LLMProvider
proveedor-extractor-MVP: OpenAI
modelo-extractor-MVP: gpt-4o-mini
canal-inicial: Gmail
integracion-jev: API-REST
jev-base-url: https://www.jevai.org
jev-endpoint-MVP: /api/v1/decisions/route
jev-autenticacion: Bearer-token
jev-api-key-env: JEV_API_KEY
openai-api-key-env: OPENAI_API_KEY
jev-fallo: aviso-error-y-no-procesar
tipo-id: UUID
autenticacion-interna-MVP: Bearer-token
reintentos-MVP: 2
backoff-MVP: exponencial
fallback-proveedor-MVP: no
timeout-extractor-MVP: 30s
temperatura-extractor-MVP: 0
revision-manual-estado: needs_review
```

## Criterio de mantenimiento

Las modificaciones a estos atributos deben hacerse únicamente en este documento. `SPEC.md` explica el diseño técnico y `PLAN.md` organiza su implementación.

## Actualizaciones

- `CLASSIFIER_MODE` reemplaza al orquestador único: `chain` (Jev → LLM → reglas → revisión), `jev` o `llm`.
- Jev deshabilitado temporalmente (429 y clasificaciones erróneas); se usa `llm` con `gpt-4o-mini`. Detalle en `../03-CLASSIFIER.md`.
- La especificación original (`SPEC.md`) describe Jev como orquestador; el clasificador LLM conserva el mismo contrato de decisión vía `classifier.RouteDecision`.
