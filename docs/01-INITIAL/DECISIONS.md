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
