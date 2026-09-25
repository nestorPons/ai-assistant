# SPEC.md: AI Task Orchestrator & Multi-Channel Listener

Decisiones compartidas: [DECISIONS.md](DECISIONS.md).

> Nota de vigencia: este documento describe el diseño original con **Jev** como orquestador único. La clasificación quedó abstraída tras `classifier.Classifier` con cadena de fallback (`CLASSIFIER_MODE`). Jev está deshabilitado temporalmente y el clasificador activo es un LLM OpenAI. Ver [docs/03-CLASSIFIER.md](../03-CLASSIFIER.md) y las actualizaciones en `DECISIONS.md`.

## 1. Visión General
Sistema backend desacoplado escrito en **Go** que monitoriza múltiples fuentes de comunicación (Gmail, WhatsApp, Telegram), normaliza los mensajes entrantes, analiza su contenido mediante modelos de IA con salida estructurada (JSON Schema) para detectar asignaciones de tareas y las persiste en una base de datos relacional. Los proveedores externos se integran exclusivamente mediante adaptadores internos, de modo que el dominio no depende de OpenAI, DeepSeek ni de otro SDK concreto.

La capa de presentación (Dashboard/CRUD) queda explícitamente **desacoplada**, permitiendo conectar cualquier panel de administración o API consumidora en una fase posterior.

Nota de proceso: la implementación funcional completa del backend no se realizará en una sola tanda. El documento de especificación define el diseño base, los contratos, las decisiones de seguridad y la arquitectura técnica; la construcción real del proyecto será secuencial y se ejecutará cuando estas especificaciones estén cerradas y validadas.

## 2. Arquitectura del Sistema

```
[ Gmail API / IMAP ] ---\
[ WhatsApp (whatsmeow) ] --> [ Ingestion Worker (Go Goroutines) ]
[ Telegram Bot API ] ---/                  |
      v
   [ User Authorization Filter ]
      |
      v
    [ Message Normalizer ]
      |
      v
    [ llm-anonymizer ]
      |
      v
    [ Context Builder ]
               |
               v
          [ Jev: Classifier / Router ]
            /          |           \
           v           v            v
         [DB Action] [LLM Extractor] [Discard]
           \           |            /
            \          v           /
             [ Relational DB ]
               ^
               |
          [ Dashboard / Frontend (Decoupled) ]
```

### Componentes Principales:
*   **Ingestion Workers (Go):** Procesos concurrentes en ejecuciones paralelas (*goroutines*) escuchando eventos en tiempo real o por *polling*.
*   **Filtro de Usuarios Autorizados:** Lista blanca configurable por canal e identificador. Solo los usuarios indicados explícitamente por el propietario pasan al procesamiento; el resto se excluye por defecto antes de guardar o analizar el contenido.
*   **Normalizador de Mensajes:** Capa que transforma cualquier evento heterogéneo a una estructura única (`IncomingMessage`).
*   **`llm-anonymizer`:** Microservicio interno que detecta y reemplaza PII (nombres, emails, teléfonos, direcciones, identificadores y otros datos sensibles) antes de enviar información a Jev o a cualquier LLM. El contenido original queda restringido a la persistencia interna autorizada.
*   **Constructor de Contexto:** Prepara un contexto controlado para el análisis, incluyendo únicamente la información necesaria del canal y del cliente. El mensaje original se conserva íntegro y claramente delimitado para evitar que el contexto altere o desvirtúe su significado.
*   **Orquestador Jev:** Servicio externo de decisiones consumido mediante API REST. Determina el enrutamiento y las condiciones de seguridad del procesamiento; no ejecuta herramientas ni sustituye al extractor de frontera.
*   **Extractor de frontera:** Modelo OpenAI consumido mediante API y un adaptador `LLMProvider` en el MVP. Solo recibe contenido sanitizado y devuelve la especificación estructurada de la tarea cuando Jev determina que debe extraerse.
*   **Capa de proveedores LLM:** Interfaces internas y adaptadores independientes para conectar varios proveedores sin exponer sus contratos al dominio, al clasificador ni al extractor.
*   **Persistencia (DB):** Base de datos relacional que consolida clientes, traza mensajes originales y almacena tareas procesadas.
*   **Capa de Presentación (Pendiente / Desacoplada):** Interfaz para consultar y editar tareas mediante el consumo directo de la DB o API interna.

---

## 3. Ingesta y Normalización de Datos

Antes de normalizar y persistir el contenido completo, el evento debe superar el filtro de usuarios autorizados. La autorización se resuelve mediante una lista blanca por canal e identificador:

*   Gmail: email autorizado.
*   WhatsApp: número de teléfono autorizado.
*   Telegram: ID o username autorizado.

Los usuarios no incluidos o desactivados se excluyen por defecto. Sus mensajes no se guardan completos, no se envían a servicios externos y no llegan a Jev ni al LLM extractor. Como máximo, se registra un evento técnico mínimo sin contenido personal para diagnóstico.

Cualquier canal entrante debe ser transformado a la siguiente estructura interna en Go antes de ser procesado por el motor de IA:

```go
type IncomingMessage struct {
    ID               string    `json:"id"`                 // ID único del canal (Message-ID, Telegram Update ID, Hash)
    Source           string    `json:"source"`             // "whatsapp", "telegram", "gmail"
    ClientIdentifier string    `json:"client_identifier"`  // Email, teléfono (+34...) o Username
    ClientName       string    `json:"client_name"`        // Nombre disponible en el canal
    RawContent       string    `json:"raw_content"`        // Contenido textual íntegro
    ReceivedAt       time.Time `json:"received_at"`
}
```

  ### 3.1. Ofuscación de datos personales

  Después de comprobar que el usuario está autorizado y antes de construir el contexto o invocar cualquier modelo, `core-engine` llama al microservicio interno `llm-anonymizer`, que ejecuta la ofuscación de datos personales.

  Esta capa debe:

  1.  Detectar PII en `ClientName`, `ClientIdentifier` y `RawContent`, incluyendo nombres, emails, teléfonos, direcciones, documentos, cuentas y otros identificadores sensibles.
  2.  Sustituir cada dato por un marcador estable y no reversible para el proveedor externo, por ejemplo `<PERSON_1>`, `<EMAIL_1>` o `<PHONE_1>`.
  3.  Crear una representación separada para IA, como `AIContent`, sin datos personales innecesarios.
  4.  Mantener el mensaje original únicamente en la base de datos interna, con acceso restringido, cifrado en reposo cuando sea posible y nunca dentro de logs.
  5.  Evitar que los datos ofuscados puedan reconstruirse a partir de prompts, respuestas, trazas o métricas.
  6.  Permitir restaurar un valor solo dentro del sistema y únicamente cuando sea necesario para una acción autorizada sobre la base de datos.

  La ofuscación debe ser determinista dentro del mismo mensaje o conversación cuando Jev necesite reconocer que dos referencias apuntan a la misma persona, pero no debe reutilizar identificadores sensibles como tokens enviados al proveedor de IA. Si el sistema no puede identificar con seguridad un dato personal, debe tratarlo como sensible y ofuscarlo.

---

## 4. Pipeline de Procesamiento de IA

El procesamiento se divide en dos fases. Primero, **Jev** recibe una petición de decisión mediante su API REST y determina si el flujo puede continuar, requiere revisión o debe bloquearse. Después, solo si la decisión permite continuar, el extractor OpenAI devuelve los datos mediante **Structured Outputs (JSON Schema)**. Antes de invocar a cualquiera de los dos se comprueba que el usuario está autorizado, se ofuscan los datos personales y se construye un contexto controlado. Jev y el extractor reciben la representación ofuscada, nunca el contenido personal original.

### 4.0. Capa de abstracción de proveedores LLM

El sistema no utilizará directamente los SDK ni los tipos de respuesta de un proveedor externo desde `internal/classifier`, `internal/extractor` o el dominio. Todas las llamadas pasarán por una interfaz interna estable y por un adaptador concreto:

```go
type LLMProvider interface {
  Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error)
}

type CompletionRequest struct {
  Model        string
  SystemPrompt string
  UserPrompt   string
  JSONSchema   []byte
  Temperature  float32
}

type CompletionResponse struct {
  Content      []byte
  Provider     string
  Model        string
  FinishReason string
  Usage        Usage
}
```

Los adaptadores concretos vivirán fuera del dominio, por ejemplo `OpenAIProvider`, `DeepSeekProvider` u otros futuros proveedores. Su responsabilidad será traducir `CompletionRequest` al protocolo externo, autenticar la petición, aplicar timeouts y convertir la respuesta al contrato interno. El resto del sistema solo dependerá de `LLMProvider`.

La selección del adaptador y del modelo se realizará mediante configuración, no mediante condicionales repartidos por el código. En el MVP se configurarán:

*   `Jev`, como servicio de decisiones externo mediante `JevAPIProvider`.
*   `OpenAI`, como proveedor del extractor estructurado mediante `OpenAIProvider`.

Ambos roles podrán usar el mismo proveedor o proveedores distintos. Un proveedor externo nunca recibirá `RawContent`, la `MappingTable`, credenciales de otros proveedores ni datos personales sin sanitizar.

El adaptador debe normalizar errores externos a categorías internas (`authentication`, `rate_limit`, `timeout`, `unavailable`, `invalid_response` y `unknown`) para que los reintentos y la revisión no dependan del proveedor utilizado. Las respuestas se validarán contra el esquema interno después de abandonar el adaptador.

### 4.1. Construcción de contexto

El `Context Builder` crea un sobre de análisis con los metadatos no sensibles, la representación ofuscada del cliente, la fecha, el estado conocido y el mensaje ofuscado delimitado. El contexto debe:

1.  Mantener `RawContent` como la fuente de verdad interna, sin enviarlo a proveedores externos ni mezclar instrucciones del sistema con el texto del usuario.
2.  Indicar a Jev que el contenido delimitado es el mensaje que debe analizar, no una instrucción de configuración.
3.  Evitar añadir información no verificada o inferida que pueda cambiar la intención del mensaje.
4.  Permitir que el contexto se amplíe con datos de la conversación cuando sea necesario, manteniendo siempre la separación entre contexto y mensaje original.

### 4.2. Protección de datos durante el procesamiento

La cadena de procesamiento debe cumplir este orden obligatorio:

1.  Identificar el canal y el usuario de origen.
2.  Comprobar que el usuario está activo en la lista de autorizados.
3.  Normalizar el evento autorizado.
4.  Guardar el original únicamente en la persistencia interna protegida.
5.  Ofuscar los datos personales para crear la representación destinada a IA.
6.  Enviar solo la representación ofuscada y el contexto mínimo a Jev o al LLM extractor.
7.  Validar las respuestas y restaurar datos únicamente mediante lógica interna autorizada, nunca mediante una petición al modelo.

No se deben incluir datos personales originales en prompts, respuestas almacenadas del proveedor, logs, métricas, mensajes de error ni trazas distribuidas.

### 4.3. Clasificación y enrutamiento mediante Jev

Jev recibe el contexto construido mediante `POST https://www.jevai.org/api/v1/decisions/route`, usando `Authorization: Bearer <JEV_API_KEY>`. El cuerpo tendrá como mínimo `task`, y opcionalmente `evidence` y `constraints`. Jev devuelve una respuesta envolvente con `code`, `message` y `data`; `code = 0` indica una respuesta válida.

El contrato de respuesta de Jev esperado es:

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "decision": "proceed_fast | deep_review | split_task | block",
    "confidence": 0.86,
    "guidance": "Continuar con el procesamiento"
  }
}
```

La aplicación no delega la ejecución de herramientas ni de operaciones SQL en Jev. Traduce la decisión de Jev a una ruta interna: `proceed_fast` permite continuar, `deep_review` y `split_task` generan revisión, y `block` descarta o bloquea el procesamiento. La creación o actualización en la base de datos siempre pasa por validación interna y repositorios tipados.

Para conservar el contrato interno del sistema, la ruta posterior puede representarse así:

```json
{
  "decision": "db_action | extract | discard",
  "confidence_score": 0.95,
  "reason": "La instrucción solicita actualizar una tarea existente",
  "action": {
    "operation": "create | update | delete",
    "entity": "client | raw_message | task",
    "target_id": "id-opcional",
    "fields": {}
  }
}
```

*   **`db_action`:** El mensaje solicita una acción directa sobre los datos (por ejemplo, crear, editar o eliminar un registro). El sistema valida la operación y la ejecuta con una transacción; nunca debe ejecutar SQL generado directamente por el modelo.
*   **`extract`:** El mensaje contiene una petición o asignación de trabajo cuyos datos deben extraerse. Se envía al LLM extractor junto con el `clean_prompt` y el contexto mínimo necesario; el mensaje original permanece únicamente en la persistencia interna.
*   **`discard`:** El mensaje no requiere persistencia ni extracción (por ejemplo, cortesía, confirmación breve o conversación informal). Se conserva en `raw_messages` para auditoría, pero no genera una tarea.

Si Jev devuelve un error HTTP, `code` distinto de cero, timeout o una respuesta inválida, `core-engine` debe generar un aviso técnico de error, marcar el mensaje para revisión y detener el procesamiento de ese mensaje. No se ejecutarán acciones de base de datos ni llamadas al extractor como fallback automático.

### 4.4. Esquema del JSON de extracción esperado

Cuando Jev devuelve `extract`, el motor enviará el mensaje al extractor de frontera utilizando **Structured Outputs (JSON Schema)** para forzar una respuesta en formato JSON estricto. La descripción resume la tarea; `specifications` conserva sus requisitos operativos.

Una petición de trabajo puede llegar incompleta: es habitual que no indique fecha límite, horas estimadas, prioridad ni requisitos. La ausencia de estos parámetros **no** convierte el mensaje en descarte ni impide crear la tarea. Lo que toda petición de trabajo siempre aporta es un **tema** (`subject`): el objeto sobre el que se trabaja (proyecto, web, cliente, sistema, documento, servicio, etc.). `subject` es obligatorio cuando `is_task = true` y debe ser concreto y breve. Los parámetros que no aparezcan en el mensaje se dejan a `null` (o ausentes) y nunca se inventan.

Ejemplo: «Arreglame lo que no funciona en nestorpons.com lo antes posible» → `subject = "nestorpons.com"`, `due_date = null`, `estimated_hours = null`, `priority = "high"` (por la urgencia), `specifications` vacío.

```json
{
  "is_task": true,
  "confidence_score": 0.95,
  "task": {
    "title": "Título resumido y conciso de la tarea",
    "description": "Explicación clara del trabajo solicitado",
    "subject": "Tema del trabajo (proyecto, web, cliente, sistema)",
    "priority": "low | medium | high",
    "estimated_hours": 2,
    "due_date": "2026-10-01 | null",
    "specifications": {
      "requirements": ["Requisito funcional identificado"],
      "constraints": ["Restricción explícita"],
      "deliverables": ["Resultado esperado"],
      "acceptance_criteria": ["Condición verificable de aceptación"],
      "dependencies": ["Dependencia conocida"],
      "open_questions": ["Punto que requiere aclaración"]
    }
  }
}
```

Campos obligatorios de `task`: `title`, `description`, `subject` y `priority`. El resto (`estimated_hours`, `due_date`, `specifications`) es opcional. `due_date` usa formato ISO `YYYY-MM-DD` y vale `null` cuando el mensaje no fija fecha.

### 4.5. Reglas del Prompt (System Rules)

1.  **Análisis fiel:** Jev y el LLM extractor deben basarse en el contenido delimitado de `clean_prompt`, que conserva la intención y estructura del mensaje original sin sus datos personales.
2.  **Acciones de base de datos:** Si el mensaje solicita guardar, editar o eliminar información existente, Jev debe devolver `"decision": "db_action"` con la operación, entidad, identificador y campos que puedan determinarse con seguridad.
3.  **Extracción:** Si contiene una instrucción o petición explícita o implícita de trabajo, Jev debe devolver `"decision": "extract"`. El extractor de frontera marcará `"is_task": true` y rellenará el objeto `task`. Es obligatorio identificar el `subject` (tema del trabajo); `specifications` y los demás parámetros se incluyen solo cuando existan requisitos o condiciones identificables.
4.  **Filtro de relevancia:** Mensajes de cortesía, confirmaciones breves ("ok", "visto", "gracias") o charlas informales deben marcarse como `"decision": "discard"`; si pasan al extractor, este debe devolver `"is_task": false`.
5.  **Nivel de prioridad:** El extractor determinará la prioridad según expresiones de urgencia conservadas en `clean_prompt` (por ejemplo, "urgente", "para hoy", "cuando puedas").
6.  **Ambigüedad:** Si Jev no puede distinguir de forma fiable entre acción, extracción o descarte, debe devolver una confianza baja y el sistema debe retener el mensaje para revisión, sin ejecutar cambios destructivos.
7.  **Tema y parámetros opcionales:** Toda petición de trabajo tiene un `subject`, aunque no indique fecha ni ningún otro parámetro. La falta de `due_date`, `estimated_hours`, prioridad o `specifications` no es motivo para marcar `is_task=false`; el extractor rellena lo que pueda y deja el resto a `null`, sin inventar datos.
8.  **Privacidad:** Jev y el LLM extractor solo recibirán contenido ofuscado. No deben intentar reconstruir, solicitar ni generar datos personales reales que no sean necesarios para la tarea.

---

## 5. Modelo de Datos y Persistencia (SQL)

### 5.1. Tabla `clients`
Almacena el directorio único de contactos/clientes y determina qué contactos están autorizados para ser procesados. No es necesaria una tabla separada para los usuarios monitorizados.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `source` (ENUM: 'whatsapp', 'telegram', 'gmail')
*   `identifier` (VARCHAR) - Teléfono, email, ID o username del canal
*   `name` (VARCHAR) - Nombre o display name disponible en el canal
*   `tracked` (BOOLEAN DEFAULT FALSE) - Indica si el contacto está incluido en la lista blanca
*   `active` (BOOLEAN DEFAULT TRUE) - Permite desactivar temporalmente el contacto sin eliminarlo
*   `created_at` (TIMESTAMP)
*   `updated_at` (TIMESTAMP)
*   Restricción UNIQUE sobre (`source`, `identifier`)

### 5.2. Tabla `raw_messages`
Guarda el registro auditable de todas las comunicaciones recibidas.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `client_id` (FOREIGN KEY -> `clients.id`)
*   `source` (ENUM: 'whatsapp', 'telegram', 'gmail')
*   `external_id` (VARCHAR) - ID original del mensaje en el proveedor
*   `content` (TEXT) - Texto completo recibido
*   `processed` (BOOLEAN DEFAULT FALSE)
*   `created_at` (TIMESTAMP)
*   El contenido original debe tener acceso restringido y no debe aparecer en logs ni enviarse a proveedores externos.

### 5.3. Tabla `tasks`
Guarda las tareas procesadas y validadas por el sistema.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `client_id` (FOREIGN KEY -> `clients.id`)
*   `message_id` (FOREIGN KEY -> `raw_messages.id`)
*   `title` (VARCHAR)
*   `description` (TEXT)
*   `subject` (VARCHAR) - Tema del trabajo (proyecto, web, cliente, sistema). Obligatorio: toda petición de trabajo lo aporta.
*   `priority` (ENUM: 'low', 'medium', 'high')
*   `estimated_hours` (FLOAT / DECIMAL NULL)
*   `due_date` (DATE NULL) - Fecha límite si el mensaje la indica; NULL en caso contrario.
*   `specifications` (JSON NULL) - Requisitos, restricciones, entregables, criterios de aceptación, dependencias y preguntas abiertas extraídas del mensaje.
*   `status` (ENUM: 'pending', 'in_progress', 'completed', 'discarded' DEFAULT 'pending')
*   `ai_confidence` (FLOAT)
*   `created_at` (TIMESTAMP)
*   `updated_at` (TIMESTAMP)

---

## 6. Flujo de Trabajo (End-to-End)

1.  **Captura:** Un *Worker* de Go recibe un evento de Gmail, WhatsApp o Telegram.
2.  **Filtro de Usuario:** Busca el canal y el identificador en `clients`. Si no existe, `tracked` es `FALSE` o `active` es `FALSE`, excluye el mensaje y detiene el flujo.
3.  **Normalización:** Transforma únicamente los eventos autorizados a `IncomingMessage`.
4.  **Identificación de Cliente:** Obtiene el `client_id` del registro encontrado en `clients` mediante `Source + ClientIdentifier`. Los contactos nuevos deben darse de alta mediante una operación interna y marcarse explícitamente con `tracked = TRUE`; nunca se crean automáticamente desde un mensaje no autorizado.
5.  **Registro Protegido:** Inserta el mensaje original autorizado en `raw_messages` con acceso restringido.
6.  **Pre-filtro Ligero:** Si el mensaje tiene menos de 3 palabras (configurable), puede evitarse la llamada a Jev y clasificarse como descartable, salvo que el canal o el contexto indiquen que forma parte de una acción pendiente.
7.  **Ofuscación:** Detecta y reemplaza los datos personales para crear la representación destinada a IA.
8.  **Construcción de Contexto:** Crea el contexto con datos mínimos y contenido ofuscado, sin modificar el original almacenado internamente.
9.  **Orquestación:** Envía únicamente el contexto ofuscado a Jev y valida su respuesta contra el esquema de decisiones (`proceed_fast`, `deep_review`, `split_task` o `block`).
10. **Revisión o bloqueo:** Si Jev devuelve `"deep_review"`, `"split_task"` o `"block"`, registra el estado interno correspondiente y no ejecuta acciones automáticas.
11. **Extracción (IA):** Si Jev devuelve `"proceed_fast"` y el contexto contiene una tarea, envía la representación ofuscada al extractor OpenAI con el JSON Schema.
12. **Persistencia de Tarea:** Si el extractor devuelve `"is_task": true`, inserta un nuevo registro en `tasks` en estado `pending`.
13. **Descarte:** Si Jev devuelve `"discard"`, marca el mensaje como procesado sin crear ni modificar tareas.
14. **Consumo Exterior:** Un frontend o API externa podrá consultar la tabla `tasks` para listar, filtrar, aprobar o modificar las tareas (CRUD).

---

## 7. Infraestructura y Despliegue

El proyecto se empaqueta mediante **Docker Compose**:

*   **Servicio `core-engine`:** Binario compilado en Go con los listeners activos y el cliente de IA.
*   **Servicio `db`:** Instancia de  MariaDB.
*   **Servicio `llm-anonymizer`:** Binario estático Go en una imagen `scratch`, expuesto solo en la red interna mediante `http://llm-anonymizer:8080/api/sanitize`.
*   **Variables de entorno :** .env 

## 8. Módulo de Anonimización de Prompts en Go

El módulo de anonimización y sanitización de PII se ejecuta antes de enviar cualquier prompt a APIs externas de LLM, como OpenAI o DeepSeek. Solo recibe mensajes que ya han superado el filtro de usuarios autorizados y nunca recibe ni devuelve credenciales de proveedores.

### 8.1. Arquitectura y restricciones de despliegue

*   **Lenguaje:** Go (Golang).
*   **Compilación:** binario estático y nativo de Go con `CGO_ENABLED=0`.
*   **Dependencias prohibidas:** no se utilizarán binarios del sistema operativo, dependencias C/C++ externas, `libonnxruntime.so` ni otros artefactos que requieran carga dinámica.
*   **Servicios externos prohibidos:** el módulo no realizará llamadas gRPC/REST a servicios de terceros para detectar o anonimizar PII.
*   **Despliegue:** imagen Docker ultraligera basada en `scratch`.
*   **Endpoint interno:** `POST http://llm-anonymizer:8080/api/sanitize`.

El servicio será accesible únicamente dentro de la red interna de Docker. La aplicación principal será responsable de enviar el resultado sanitizado al proveedor LLM; el anonimizado no tendrá acceso directo a APIs de OpenAI, DeepSeek u otros proveedores.

### 8.2. Componentes técnicos

#### Contratos de integración y almacenamiento temporal

La capa de anonimización debe exponer contratos internos y explícitos para no depender de servicios externos ni de flujos implícitos:

```go
type MappingStore interface {
    Save(ctx context.Context, requestID string, mapping MappingTable, ttl time.Duration) error
    Load(ctx context.Context, requestID string) (MappingTable, error)
    Delete(ctx context.Context, requestID string) error
}
```

El almacenamiento temporal es responsabilidad del servicio de anonimización. En el MVP se usará `MemoryMappingStore` con TTL de 5 minutos. Si más adelante se requieren múltiples workers o ejecución asíncrona, puede sustituirse por `RedisMappingStore` sin cambiar la API pública del servicio.

Los contratos HTTP para el servicio deben definirse como:

```go
type SanitizeRequest struct {
    Prompt        string `json:"prompt"`
    ConversationID string `json:"conversation_id,omitempty"`
    Mode          string `json:"mode,omitempty"`
}

type SanitizeResponse struct {
    RequestID string `json:"request_id"`
    CleanPrompt string `json:"clean_prompt"`
    Entities []EntityMatch `json:"entities,omitempty"`
}

type RevertRequest struct {
    RequestID string `json:"request_id"`
    ProcessedText string `json:"processed_text"`
}

type RevertResponse struct {
    RequestID string `json:"request_id"`
    RevertedText string `json:"reverted_text"`
}
```

La autenticación de servicios internos debe limitarse a la red privada y a credenciales del propio sistema. En el MVP se recomienda `Authorization: Bearer <token-interno>` o mTLS dentro de Docker; cualquier otra ruta debe quedar fuera del alcance del servicio.

#### Inferencia contextual NER

Para detectar entidades que no puedan identificarse con patrones exactos se utilizará:

*   Motor ONNX: `github.com/owulveryck/onnx-go`.
*   Backend: `github.com/owulveryck/onnx-go/backend/x/gorgonnx`.
*   Modelo principal: `Davide/xlm-roberta-base-finetuned-panx-ner`, convertido y validado en formato ONNX.
*   Modelo alternativo: `mrm8488/bert-spanish-cased-finetuned-ner`, sujeto a la misma validación de exportación y calidad.
*   Artefactos obligatorios: `model.onnx` y `tokenizer.json`.
*   Etiquetas mínimas utilizadas: `PER` para personas, `ORG` para organizaciones y `LOC` para ubicaciones.

El modelo principal se selecciona por su cobertura multilingüe para español, catalán/valenciano e inglés. Esta cobertura se verificará con un conjunto de evaluación propio antes de considerarla suficiente para producción; los patrones deterministas seguirán teniendo prioridad para emails, teléfonos, documentos, IBAN y tarjetas.

La inferencia debe ejecutarse dentro del proceso Go mediante código compatible con la compilación estática y sin depender de runtimes nativos externos. El modelo detectará, como mínimo, nombres de personas, ubicaciones y organizaciones.

#### Tokenización nativa

Se utilizará `github.com/sugarme/tokenizer` para leer `tokenizer.json` y generar los tensores `input_ids` y `attention_mask`. La tokenización será completamente nativa de Go y no utilizará CGO.

#### Detección estructurada y heurística

La detección rápida se implementará en Go mediante:

*   `regexp` para emails, teléfonos y patrones de documentos.
*   Validadores algebraicos para DNI/NIE mediante módulo 23, IBAN, tarjetas mediante algoritmo de Luhn y otros formatos definidos por el dominio.
*   Aho-Corasick mediante `github.com/BobuSumisu/aho-corasick` para búsquedas masivas en memoria de diccionarios y listas configuradas.

Los resultados de las fases heurística y contextual se combinarán, deduplicarán y ordenarán por posición antes de aplicar las sustituciones.

### 8.3. Flujo de datos

1. **Petición de entrada:** la aplicación principal, Laravel, Node.js o HTMX, recibe la interacción del usuario y realiza un `POST` al microservicio interno.
2. **Fase 1, detección rápida:** se ejecutan regex, validadores exactos y autómatas sobre el texto.
3. **Fase 2, detección contextual:** se tokeniza el texto y se ejecuta la inferencia NER con ONNX nativo para detectar entidades dependientes del contexto.
4. **Fase 3, resolución de solapamientos:** se consolidan las entidades detectadas, priorizando coincidencias más específicas y evitando reemplazar dos veces el mismo rango.
5. **Fase 4, sanitización:** se reemplazan las entidades por tokens etiquetados, como `{{DNI_1}}`, `{{PERSONA_1}}`, `{{EMAIL_1}}`, `{{TELEFONO_1}}`, `{{UBICACION_1}}` u `{{ORGANIZACION_1}}`.
6. **Respuesta:** el servicio devuelve el JSON con `request_id`, `clean_prompt` y metadatos técnicos no sensibles. La tabla de correspondencias nunca se devuelve al cliente.
7. **Consumo LLM:** la aplicación cliente envía únicamente `clean_prompt` al proveedor del LLM.

El objetivo operativo es baja latencia y procesamiento local. La latencia real dependerá del tamaño del prompt, del modelo cargado y del hardware; no se considerará garantizado un tiempo inferior a un milisegundo para la inferencia NER.

### 8.4. Contrato HTTP

#### Petición

```http
POST /api/sanitize
Content-Type: application/json
```

```json
{
  "prompt": "Texto original autorizado que será enviado a un LLM",
  "conversation_id": "id-interno-opcional",
  "mode": "prompt"
}
```

`prompt` es obligatorio. `conversation_id` permite mantener la consistencia de los marcadores dentro de una conversación, pero nunca debe contener PII ni exponerse al proveedor externo. `mode` permite distinguir el prompt completo de otros textos compatibles con el mismo servicio.

#### Respuesta correcta

```json
{
  "request_id": "uuid-v4",
  "clean_prompt": "Texto con {{PERSONA_1}} y {{EMAIL_1}} sustituidos",
  "entities": [
    {
      "type": "PERSON",
      "token": "{{PERSONA_1}}",
      "start": 10,
      "end": 25
    }
  ]
}
```

`request_id` es un UUIDv4 único por petición y permite solicitar la reversión dentro del TTL. `entities` es opcional para el consumidor y no debe incluir nunca el valor original detectado. Las posiciones corresponden al texto de entrada y no deben utilizarse para reconstruir PII.

#### Errores

El servicio debe devolver códigos HTTP adecuados (`400` para peticiones inválidas, `401` para credenciales ausentes o inválidas, `403` para servicios no autorizados, `404` para un `request_id` inexistente o expirado, `413` para prompts demasiado grandes, `429` para límites internos y `500`/`503` para errores de disponibilidad) sin incluir el prompt, entidades originales ni datos sensibles en el cuerpo o en los logs.

El cuerpo de error debe tener una forma estable y no sensible:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "La petición no cumple el contrato"
  }
}
```

Los mensajes públicos deben ser genéricos. Los detalles técnicos se conservarán solo en logs internos estructurados y redactados.

### 8.5. Reglas de anonimización y seguridad

*   El contenido original se procesa en memoria y no se persiste como prompt en el microservicio. Los valores necesarios para la reversión se conservan únicamente en una `MappingTable` temporal.
*   No se registran prompts, respuestas completas, valores detectados ni tokens que permitan inferirlos.
*   Si varias apariciones representan el mismo dato dentro de una conversación o petición, deben recibir el mismo marcador estable; el marcador no puede derivarse del valor real.
*   Si una entidad es ambigua o no puede clasificarse con seguridad, se tratará como sensible y se anonimizará.
*   Las reglas heurísticas tendrán prioridad para patrones inequívocos como tarjetas, IBAN, emails y documentos válidos.
*   La reversión solo estará disponible para servicios internos autenticados y autorizados; no será una operación pública para usuarios finales ni para el proveedor LLM.
*   Se aplicarán límites de tamaño, timeouts, validación de `Content-Type`, controles de acceso a la red interna y protección contra la reutilización indebida de `request_id`.
*   La salida sanitizada se validará antes de que la aplicación principal la envíe al proveedor LLM.

### 8.6. Reversión y tabla de correspondencias

La reversión permite almacenar en la base de datos interna o presentar al usuario los valores reales después de que el LLM haya procesado el texto anonimizado. El LLM nunca recibe la tabla de correspondencias ni puede solicitarla.

#### Estrategia de tokens

La anonimización utiliza sustitución determinista mediante tokens etiquetados con el formato `{{TIPO_N}}`. `TIPO` identifica la clase de entidad y `N` es un índice controlado por la petición o conversación. Ejemplos: `{{PERSONA_1}}`, `{{DNI_1}}` y `{{EMAIL_1}}`.

#### Gestión de estado

Por cada petición de `/api/sanitize`, el microservicio genera internamente:

```go
type MappingTable map[string]string
```

El mapa asocia cada token con el valor PII original en texto plano. Esta información nunca aparece en la respuesta HTTP, logs, métricas, trazas ni prompts enviados al LLM.

*   **Flujo síncrono:** la tabla se conserva en RAM, indexada por un `request_id` UUIDv4, hasta completar la operación de reversión o expirar el TTL de 5 minutos.
*   **Flujo asíncrono:** la tabla se almacena en Redis con TTL de 5 minutos, usando una clave separada por `request_id`. Redis debe estar en la red interna, con autenticación, acceso restringido y cifrado en tránsito cuando sea posible.
*   **Expiración y errores:** al expirar el TTL, reiniciarse el servicio o fallar la recuperación, la reversión debe responder `404` sin revelar si existieron valores concretos.
*   **Aislamiento:** el `request_id` por sí solo no concede autorización; `/api/revert` requiere autenticación del servicio llamador y autorización para ese flujo.

#### Límites operativos del MVP

*   `Content-Type` obligatorio: `application/json`.
*   Tamaño máximo de `prompt` y `processed_text`: 32 KiB por petición, configurable mediante variable de entorno.
*   Tiempo máximo de procesamiento: 10 segundos por petición.
*   TTL de la `MappingTable`: 5 minutos desde su creación.
*   El `request_id` solo puede utilizarse para una reversión exitosa; después se elimina la tabla.
*   Las peticiones concurrentes para el mismo `request_id` deben serializarse o devolver un error controlado, sin duplicar la reversión.

#### Endpoint de reversión

```http
POST /api/revert
Content-Type: application/json
```

```json
{
  "request_id": "uuid-v4",
  "processed_text": "Asignar la tarea a {{PERSONA_1}} y notificar a {{EMAIL_1}}"
}
```

El servicio recupera la `MappingTable`, sustituye únicamente tokens conocidos y devuelve el texto con los valores reales:

```json
{
  "request_id": "uuid-v4",
  "reverted_text": "Asignar la tarea a María García y notificar a maria@example.com"
}
```

La aplicación principal solo podrá persistir o renderizar `reverted_text` después de validar que la respuesta procede de una llamada autorizada. El endpoint debe rechazar tokens desconocidos o malformados, limitar el tamaño de la entrada y evitar incluir valores PII en mensajes de error. La tabla debe eliminarse tras una reversión exitosa cuando el flujo no requiera más de una lectura.

### 8.7. Artefactos y validación de compilación

El contenedor debe incluir únicamente el binario estático y los artefactos de inferencia necesarios (`model.onnx`, `tokenizer.json` y diccionarios). La imagen final no debe contener compiladores, shells, gestores de paquetes ni bibliotecas compartidas.

La construcción debe verificar, como mínimo:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /llm-anonymizer ./cmd/llm-anonymizer
```

También se deben ejecutar pruebas unitarias de regex, validadores, Aho-Corasick, tokenización, resolución de solapamientos, sustitución y `MappingTable`; pruebas de integración de `/api/sanitize` y `/api/revert`; expiración de TTL, autorización, eliminación tras reversión y una comprobación de que ningún valor original aparece en respuestas no autorizadas, logs o errores.