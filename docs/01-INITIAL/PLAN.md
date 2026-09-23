# PLAN.md: AI Task Orchestrator & Multi-Channel Listener

Decisiones compartidas: [DECISIONS.md](DECISIONS.md).

## 1. Objetivo

Implementar un backend desacoplado en Go que reciba mensajes de Gmail, WhatsApp y Telegram, filtre primero los usuarios autorizados, normalice sus mensajes, construya un contexto controlado, los clasifique mediante Jev y los dirija a una de estas rutas:

- Acción de guardado o edición en la base de datos.
- Extracción estructurada mediante un LLM.
- Descarte del mensaje cuando no requiera procesamiento.

El sistema conservará siempre el mensaje original de los usuarios autorizados para auditoría y permitirá conectar posteriormente un dashboard o API externa.

Solo se hará seguimiento de los usuarios que el propietario indique explícitamente. El resto quedará excluido por defecto antes de guardar o analizar cualquier mensaje.

## 2. Fases de Implementación

### Fase 0. Cierre documental y preparación de implementación

- Cerrar la primera versión final de `SPEC.md` y `PLAN.md` antes de arrancar la construcción funcional.
- Fijar Jev como orquestador y definir el modelo frontera inicial del extractor, los adaptadores de proveedores, `Davide/xlm-roberta-base-finetuned-panx-ner` como modelo NER principal, `tokenizer.json` compatible y la política de autenticación interna.
- Mantener el dominio independiente del proveedor mediante una interfaz `LLMProvider`; los SDK externos solo podrán usarse dentro de adaptadores concretos.
- Usar almacenamiento temporal en RAM para `MappingTable` en el MVP; reservar Redis para una futura ejecución multiinstancia o asíncrona.
- Concretar los contratos HTTP de `/api/sanitize` y `/api/revert`, incluidos errores, TTL y permisos.
- Definir el esquema SQL definitivo para MariaDB y los índices cruciales antes de crear migraciones y repositorios.
- Preparar la lista de tareas técnicas pendientes y distinguir lo que es diseño de arquitectura de lo que es construcción real del sistema.

La implementación completa del backend no se ejecutará en una sola entrega. Esta fase de cierre documental sirve para dejar fijadas las decisiones de diseño antes de comenzar la creación real del proyecto.

### Fase 1. Base del proyecto Go

- Crear el módulo Go y la estructura inicial:
  - `cmd/core-engine`
  - `internal/ingestion`
  - `internal/normalizer`
  - `internal/context`
  - `internal/classifier`
  - `internal/extractor`
  - `internal/persistence`
  - `internal/domain`
- Configurar variables de entorno, logging y manejo de errores.
- Añadir Docker Compose con `core-engine`, `llm-anonymizer` y MariaDB.

### Fase 2. Modelo de dominio y base de datos

- Definir las estructuras `IncomingMessage`, `ClassificationResult`, `ExtractionResult`, `Task` y `Client`.
- Crear migraciones para las tablas `clients`, `raw_messages` y `tasks`.
- Añadir a `clients` los campos `source`, `identifier`, `name`, `tracked`, `active`, `created_at` y `updated_at`; usar una restricción única sobre (`source`, `identifier`).
- Definir en `tasks` los campos `title`, `description`, `priority`, `estimated_hours`, `specifications` (JSON), `status`, `ai_confidence`, `created_at` y `updated_at`.
- Añadir índices y restricciones de unicidad para evitar mensajes duplicados, especialmente `raw_messages(source, external_id)`.
- Definir un `MappingStore` para la reversión interna del anonimizador con implementaciones `MemoryMappingStore` y, en futuro, `RedisMappingStore`.
- Implementar repositorios con consultas parametrizadas y transacciones.

### Fase 3. Filtro de usuarios autorizados

Este filtro debe ejecutarse antes de normalizar completamente el mensaje, persistirlo o enviarlo a cualquier modelo de IA.

- Usar `clients` como lista blanca, autorizando únicamente registros con `tracked = TRUE` y `active = TRUE`:
  - Email para Gmail.
  - Número de teléfono para WhatsApp.
  - ID o username para Telegram.
- Denegar por defecto cualquier usuario que no aparezca en `clients`, tenga `tracked = FALSE` o esté inactivo.
- Permitir activar y desactivar usuarios sin eliminar su historial.
- Evitar guardar el contenido completo de mensajes de usuarios no autorizados.
- No enviar mensajes no autorizados a Jev, al LLM extractor ni a ningún servicio externo.
- Registrar únicamente un evento técnico mínimo de exclusión si es necesario para diagnóstico, sin contenido del mensaje.
- Añadir una interfaz de administración o configuración inicial para que el propietario cree o actualice clientes y marque explícitamente cuáles deben ser rastreados.

### Fase 4. Ingesta de mensajes

Implementar inicialmente Gmail como único canal para reducir la complejidad.

- Crear el listener del canal.
- Aplicar el filtro de usuarios autorizados inmediatamente al recibir el evento.
- Convertir únicamente los mensajes autorizados a `IncomingMessage`.
- Resolver el cliente autorizado mediante `source + ClientIdentifier`; los contactos nuevos deben darse de alta previamente mediante una operación interna con `tracked = TRUE`.
- Guardar siempre el mensaje original autorizado en `raw_messages`.
- Añadir idempotencia usando `source + external_id`.

Después incorporar los demás canales:

1. Gmail/IMAP.
2. Telegram.
3. WhatsApp mediante `whatsmeow`.

### Fase 5. Constructor de contexto

- Crear el `ContextBuilder`.
- Crear una capa `PIIRedactor` o `PersonalDataObfuscator` antes de construir el contexto para IA.
- Delegar la sanitización efectiva en `llm-anonymizer` mediante `POST /api/sanitize`; `core-engine` no enviará prompts directamente a proveedores externos antes de recibir `clean_prompt`.
- Detectar y reemplazar nombres, emails, teléfonos, direcciones, documentos, cuentas y otros datos personales.
- Generar marcadores estables como `<PERSON_1>`, `<EMAIL_1>` o `<PHONE_1>` sin enviar los valores reales al proveedor de IA.
- Mantener el contenido original únicamente en la persistencia interna protegida y fuera de logs, métricas y trazas.
- Separar claramente las instrucciones del sistema, los metadatos no sensibles y el contenido ofuscado.
- Mantener `RawContent` intacto.
- Evitar que el mensaje sea interpretado como instrucciones del sistema.
- Preparar soporte futuro para contexto conversacional.

### Fase 5.1. Microservicio de anonimización de prompts

- Crear `cmd/llm-anonymizer` como microservicio HTTP interno en Go.
- Compilar con `CGO_ENABLED=0`, `GOOS=linux` y `GOARCH=amd64` como binario estático.
- Implementar `POST /api/sanitize` con validación de entrada, límites de tamaño y respuestas JSON sin PII.
- Generar un `request_id` UUIDv4 por petición y devolverlo junto con `clean_prompt`, sin devolver la tabla de correspondencias.
- Crear la primera fase de detección con `regexp`, validadores de DNI/NIE, IBAN, tarjetas mediante Luhn, emails y teléfonos.
- Integrar Aho-Corasick para diccionarios y listas configuradas en memoria.
- Definir los contratos HTTP, también en Go, para `SanitizeRequest`, `SanitizeResponse`, `RevertRequest` y `RevertResponse`.
- Definir el modelo del `MappingStore` (`Save`, `Load`, `Delete`) y escoger `MemoryMappingStore` como implementación base del MVP.
- Convertir y validar `Davide/xlm-roberta-base-finetuned-panx-ner` como `model.onnx`; conservar `mrm8488/bert-spanish-cased-finetuned-ner` como alternativa evaluable.
- Incorporar `model.onnx` y `tokenizer.json` en la imagen del servicio y validar las etiquetas `PER`, `ORG` y `LOC` para español, catalán/valenciano e inglés.
- Integrar `onnx-go`, `gorgonnx` y `sugarme/tokenizer` para NER contextual sin CGO ni runtimes nativos externos.
- Empaquetar el servicio en una imagen Docker basada en `scratch` con el binario, el modelo ONNX, `tokenizer.json` y diccionarios.
- Consolidar resultados, resolver solapamientos y reemplazar PII por tokens etiquetados deterministas como `{{PERSONA_1}}` o `{{DNI_1}}`.
- Implementar `MappingTable` por petición, con almacenamiento en RAM para flujos síncronos o Redis con TTL de 5 minutos para flujos asíncronos.
- Implementar `POST /api/revert` para restaurar tokens en texto procesado únicamente para servicios internos autenticados y autorizados.
- Eliminar la tabla tras una reversión exitosa cuando el flujo no requiera más de una lectura.
- Mantener consistencia de tokens mediante `conversation_id` sin enviar identificadores sensibles al proveedor LLM.
- Garantizar que prompts, entidades originales y datos sensibles no aparezcan en logs, errores, métricas ni respuestas.
- Probar la compilación estática, ambos endpoints, los validadores, la tokenización, la inferencia, la expiración del TTL, la autorización y la ausencia de PII en respuestas, logs y errores.

### Fase 6. Clasificación con Jev

- Definir `LLMProvider` y los tipos internos de petición, respuesta y error sin referencias a SDK externos.
- Implementar el registro y la selección por configuración de adaptadores de proveedores LLM.
- Implementar `JevAPIProvider` contra `POST https://www.jevai.org/api/v1/decisions/route` con Bearer token.
- Mapear `proceed_fast`, `deep_review`, `split_task` y `block` a rutas internas sin permitir que Jev ejecute herramientas ni SQL.
- Generar un aviso técnico y detener el mensaje cuando Jev falle, agote el timeout o devuelva una respuesta inválida.
- Definir una interfaz independiente del proveedor:

```go
type Classifier interface {
    Classify(ctx context.Context, input ClassificationInput) (ClassificationResult, error)
}
```

- Implementar el adaptador OpenAI para el extractor de frontera.
- Validar su respuesta contra JSON Schema.
- Soportar las decisiones `db_action`, `extract` y `discard`.
- Establecer umbrales de confianza.
- Enviar a revisión los casos ambiguos.

### Fase 7. Acciones sobre la base de datos

- Validar la operación devuelta por Jev.
- Permitir inicialmente solo crear y actualizar registros.
- No ejecutar SQL generado por el modelo.
- Convertir las respuestas del modelo a comandos internos tipados.
- Ejecutar los cambios mediante repositorios y transacciones.
- Registrar la auditoría de cada acción realizada.

### Fase 8. Extracción estructurada

- Definir la interfaz del extractor:

```go
type Extractor interface {
    Extract(ctx context.Context, input ExtractionInput) (ExtractionResult, error)
}
```

- Implementar el adaptador del proveedor elegido para el extractor y enviarle únicamente los mensajes autorizados y clasificados como `extract`.
- Validar la respuesta contra JSON Schema.
- Extraer título, descripción, prioridad, horas estimadas y las especificaciones de la tarea.
- Persistir `tasks.specifications` como JSON con requisitos, restricciones, entregables, criterios de aceptación, dependencias y preguntas abiertas.
- Crear la tarea en estado `pending`.
- Guardar la confianza del modelo.

### Fase 9. Descarte y procesamiento final

- Marcar como procesados los mensajes autorizados clasificados como `discard`.
- No crear tareas para mensajes irrelevantes.
- Conservar el mensaje original para auditoría.
- Registrar el motivo y la confianza de la clasificación.

### Fase 10. Workers y concurrencia

- Crear una cola interna de procesamiento.
- Usar goroutines para procesar mensajes autorizados.
- Añadir límites de concurrencia.
- Implementar reintentos para errores temporales.
- Evitar reprocesar mensajes ya procesados.
- Añadir recuperación para mensajes fallidos.

### Fase 11. API o consumo externo

Aunque el dashboard queda desacoplado, conviene crear una API mínima para probar el sistema:

- Gestionar los clientes y sus campos `tracked` y `active` para controlar la lista de usuarios autorizados.
- Listar tareas.
- Consultar una tarea.
- Actualizar el estado.
- Editar título, descripción o prioridad.
- Consultar mensajes originales autorizados.
- Consultar errores o mensajes pendientes de revisión.

### Fase 12. Seguridad y observabilidad

- No registrar API keys ni contenido sensible en los logs.
- Validar y limitar el tamaño de los mensajes.
- Proteger las credenciales mediante variables de entorno.
- No incluir contenido de usuarios no autorizados en logs ni métricas.
- No enviar PII sin ofuscar a Jev, al LLM extractor ni a ningún servicio externo.
- Cifrar o restringir el acceso al contenido original almacenado.
- Añadir logs estructurados.
- Añadir métricas para usuarios excluidos, mensajes recibidos, descartados, acciones de BD, extracciones, errores y tiempos de procesamiento.
- Aplicar timeouts a las llamadas externas.

### Fase 13. Pruebas

Crear pruebas unitarias para:

- Filtro de usuarios autorizados.
- Ofuscación de datos personales antes de cualquier llamada a IA.
- Activación y desactivación de usuarios.
- Normalización por canal.
- Construcción de contexto.
- Validación de respuestas de Jev.
- Validación del JSON del extractor.
- Clasificación de mensajes descartables.
- Acciones de creación y actualización.
- Idempotencia.
- Persistencia de tareas.

Crear pruebas de integración para:

- MariaDB.
- Flujo completo desde mensaje autorizado hasta tarea.
- Exclusión de mensajes de usuarios no autorizados.
- Fallos y reintentos.
- Respuestas inválidas del modelo.

## 3. MVP Recomendado

El primer entregable debe incluir únicamente:

1. Go.
2. MariaDB.
3. Un canal de entrada.
4. Lista blanca de usuarios autorizados.
5. Persistencia protegida de clientes y mensajes autorizados.
6. `llm-anonymizer` con almacenamiento temporal en RAM.
7. `ContextBuilder`.
8. Clasificación mediante Jev.
9. Extracción mediante un LLM estructurado.
10. Creación de tareas.
11. Pruebas del flujo completo.
12. Docker Compose con `core-engine`, `llm-anonymizer` y MariaDB.

Después del MVP se incorporarán las acciones de edición, los demás canales y la API externa.

## 4. Criterios de Finalización del MVP

El MVP se considerará terminado cuando:

- Un mensaje real de un canal pueda transformarse en `IncomingMessage`.
- Un usuario no autorizado sea excluido antes de guardar o analizar su mensaje.
- Un usuario autorizado pueda activarse o desactivarse mediante configuración.
- El mensaje original autorizado se guarde sin modificaciones.
- Jev y el LLM extractor solo reciban contenido ofuscado.
- Los datos personales originales no aparezcan en logs, métricas ni trazas.
- Jev devuelva una clasificación validada.
- Los mensajes descartables no creen tareas.
- Los mensajes de trabajo generen una tarea con JSON válido.
- Las especificaciones extraídas se guarden en `tasks.specifications` sin perder requisitos, restricciones ni criterios de aceptación.
- Las acciones de guardado o edición se ejecuten mediante repositorios y transacciones.
- Los mensajes duplicados no se procesen dos veces.
- Los errores del LLM y del canal puedan reintentarse o quedar registrados.
- El sistema pueda levantarse mediante Docker Compose.
- El flujo principal tenga pruebas automatizadas.

## 5. Orden Recomendado de Ejecución

1. Inicializar el proyecto Go y Docker Compose.
2. Crear el esquema SQL y los repositorios.
3. Implementar la lista blanca mediante `clients.tracked` y `clients.active`.
4. Implementar un adaptador de entrada con mensajes simulados.
5. Verificar el filtro antes de persistir mensajes.
6. Implementar `IncomingMessage` y la normalización.
7. Implementar el `ContextBuilder`.
8. Implementar Jev con un cliente simulado para pruebas.
9. Implementar las rutas `discard`, `db_action` y `extract`.
10. Integrar el LLM extractor.
11. Añadir pruebas unitarias e integración.
12. Conectar el primer canal real.
13. Añadir workers, reintentos y observabilidad.
14. Incorporar los canales restantes y la API externa.
