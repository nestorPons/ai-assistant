# PLAN.md: AI Task Orchestrator & Multi-Channel Listener

## 1. Objetivo

Implementar un backend desacoplado en Go que reciba mensajes de Gmail, WhatsApp y Telegram, filtre primero los usuarios autorizados, normalice sus mensajes, construya un contexto controlado, los clasifique mediante Jev y los dirija a una de estas rutas:

- Acción de guardado o edición en la base de datos.
- Extracción estructurada mediante un LLM.
- Descarte del mensaje cuando no requiera procesamiento.

El sistema conservará siempre el mensaje original de los usuarios autorizados para auditoría y permitirá conectar posteriormente un dashboard o API externa.

Solo se hará seguimiento de los usuarios que el propietario indique explícitamente. El resto quedará excluido por defecto antes de guardar o analizar cualquier mensaje.

## 2. Fases de Implementación

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
- Añadir Docker Compose con `core-engine` y MariaDB.

### Fase 2. Modelo de dominio y base de datos

- Definir las estructuras `IncomingMessage`, `ClassificationResult`, `ExtractionResult`, `Task` y `Client`.
- Crear migraciones para las tablas `clients`, `raw_messages` y `tasks`.
- Crear una tabla de configuración de usuarios autorizados, por ejemplo `tracked_users`, con canal, identificador, nombre opcional, estado activo y fechas de creación/actualización.
- Añadir índices y restricciones de unicidad para evitar mensajes duplicados.
- Implementar repositorios con consultas parametrizadas y transacciones.

### Fase 3. Filtro de usuarios autorizados

Este filtro debe ejecutarse antes de normalizar completamente el mensaje, persistirlo o enviarlo a cualquier modelo de IA.

- Crear una lista blanca de usuarios autorizados por canal e identificador:
  - Email para Gmail.
  - Número de teléfono para WhatsApp.
  - ID o username para Telegram.
- Denegar por defecto cualquier usuario que no aparezca en la lista o esté inactivo.
- Permitir activar y desactivar usuarios sin eliminar su historial.
- Evitar guardar el contenido completo de mensajes de usuarios no autorizados.
- No enviar mensajes no autorizados a Jev, al LLM extractor ni a ningún servicio externo.
- Registrar únicamente un evento técnico mínimo de exclusión si es necesario para diagnóstico, sin contenido del mensaje.
- Añadir una interfaz de administración o configuración inicial para que el propietario indique qué usuarios deben ser rastreados.

### Fase 4. Ingesta de mensajes

Implementar inicialmente un solo canal para reducir la complejidad, preferiblemente Telegram o Gmail.

- Crear el listener del canal.
- Aplicar el filtro de usuarios autorizados inmediatamente al recibir el evento.
- Convertir únicamente los mensajes autorizados a `IncomingMessage`.
- Resolver o crear el cliente mediante `ClientIdentifier`.
- Guardar siempre el mensaje original autorizado en `raw_messages`.
- Añadir idempotencia usando `source + external_id`.

Después incorporar los demás canales:

1. Telegram.
2. Gmail/IMAP.
3. WhatsApp mediante `whatsmeow`.

### Fase 5. Constructor de contexto

- Crear el `ContextBuilder`.
- Crear una capa `PIIRedactor` o `PersonalDataObfuscator` antes de construir el contexto para IA.
- Detectar y reemplazar nombres, emails, teléfonos, direcciones, documentos, cuentas y otros datos personales.
- Generar marcadores estables como `<PERSON_1>`, `<EMAIL_1>` o `<PHONE_1>` sin enviar los valores reales al proveedor de IA.
- Mantener el contenido original únicamente en la persistencia interna protegida y fuera de logs, métricas y trazas.
- Separar claramente las instrucciones del sistema, los metadatos no sensibles y el contenido ofuscado.
- Mantener `RawContent` intacto.
- Evitar que el mensaje sea interpretado como instrucciones del sistema.
- Preparar soporte futuro para contexto conversacional.

### Fase 6. Clasificación con Jev

- Definir una interfaz independiente del proveedor:

```go
type Classifier interface {
    Classify(ctx context.Context, input ClassificationInput) (ClassificationResult, error)
}
```

- Implementar el cliente del modelo Jev.
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

- Enviar al LLM únicamente los mensajes autorizados y clasificados como `extract`.
- Validar la respuesta contra JSON Schema.
- Extraer título, descripción, prioridad y horas estimadas.
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

- Gestionar la lista de usuarios autorizados.
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

- PostgreSQL o MariaDB.
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
6. Persistencia protegida de clientes y mensajes autorizados.
7. Ofuscación de datos personales.
8. `ContextBuilder`.
9. Clasificación mediante Jev.
10. Extracción mediante un LLM estructurado.
11. Creación de tareas.
12. Pruebas del flujo completo.
13. Docker Compose.

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
- Las acciones de guardado o edición se ejecuten mediante repositorios y transacciones.
- Los mensajes duplicados no se procesen dos veces.
- Los errores del LLM y del canal puedan reintentarse o quedar registrados.
- El sistema pueda levantarse mediante Docker Compose.
- El flujo principal tenga pruebas automatizadas.

## 5. Orden Recomendado de Ejecución

1. Inicializar el proyecto Go y Docker Compose.
2. Crear el esquema SQL y los repositorios.
3. Implementar la lista blanca de usuarios autorizados.
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
