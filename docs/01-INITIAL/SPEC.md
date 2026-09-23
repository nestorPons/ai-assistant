# SPEC.md: AI Task Orchestrator & Multi-Channel Listener

## 1. Visión General
Sistema backend desacoplado escrito en **Go** que monitoriza múltiples fuentes de comunicación (Gmail, WhatsApp, Telegram), normaliza los mensajes entrantes, analiza su contenido mediante modelos de IA (OpenAI / DeepSeek) con salida estructurada (JSON Schema) para detectar asignaciones de tareas y las persiste en una base de datos relacional. 

La capa de presentación (Dashboard/CRUD) queda explícitamente **desacoplada**, permitiendo conectar cualquier panel de administración o API consumidora en una fase posterior.

---

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
    [ PII Redactor / Obfuscator ]
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
*   **Ofuscador de Datos Personales:** Detecta y reemplaza PII (nombres, emails, teléfonos, direcciones, identificadores y otros datos sensibles) antes de enviar información a Jev o a cualquier LLM. El contenido original queda restringido a la persistencia interna autorizada.
*   **Constructor de Contexto:** Prepara un contexto controlado para el análisis, incluyendo únicamente la información necesaria del canal y del cliente. El mensaje original se conserva íntegro y claramente delimitado para evitar que el contexto altere o desvirtúe su significado.
*   **Clasificador Jev:** Modelo de IA encargado del análisis inicial y del enrutamiento del mensaje. Decide si corresponde ejecutar una acción sobre la base de datos, solicitar extracción estructurada a otro LLM o descartar el mensaje.
*   **Motor de IA (LLM Pipeline):** LLM con *Structured Outputs* (JSON Schema) que extrae los datos de negocio únicamente cuando Jev determina que el mensaje contiene información que debe persistirse.
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

  Después de comprobar que el usuario está autorizado y antes de construir el contexto o invocar cualquier modelo, se ejecuta una capa de ofuscación de datos personales.

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

El procesamiento se divide en dos fases. Primero, el modelo **Jev** clasifica y enruta el mensaje. Después, solo si la clasificación lo requiere, un LLM de extracción devuelve los datos mediante **Structured Outputs (JSON Schema)**. Antes de invocar a Jev se comprueba que el usuario está autorizado, se ofuscan los datos personales y se construye un contexto controlado. Jev y el LLM extractor reciben la representación ofuscada, nunca el contenido personal original.

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

Jev recibe el contexto construido y devuelve una decisión estructurada. El contrato mínimo esperado es:

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
*   **`extract`:** El mensaje contiene una petición o asignación de trabajo cuyos datos deben extraerse. Se envía al LLM extractor junto con el mensaje original y el contexto mínimo necesario.
*   **`discard`:** El mensaje no requiere persistencia ni extracción (por ejemplo, cortesía, confirmación breve o conversación informal). Se conserva en `raw_messages` para auditoría, pero no genera una tarea.

### 4.4. Esquema del JSON de extracción esperado

Cuando Jev devuelve `extract`, el motor enviará el mensaje al LLM utilizando **Structured Outputs (JSON Schema)** para forzar una respuesta en formato JSON estricto:

```json
{
  "is_task": true,
  "confidence_score": 0.95,
  "task": {
    "title": "Título resumido y conciso de la tarea",
    "description": "Explicación clara del trabajo solicitado",
    "priority": "low | medium | high",
    "estimated_hours": 2
  }
}
```

### 4.5. Reglas del Prompt (System Rules)

1.  **Análisis fiel:** Jev y el LLM extractor deben basarse en el mensaje original delimitado y no en una reformulación del mismo.
2.  **Acciones de base de datos:** Si el mensaje solicita guardar, editar o eliminar información existente, Jev debe devolver `"decision": "db_action"` con la operación, entidad, identificador y campos que puedan determinarse con seguridad.
3.  **Extracción:** Si contiene una instrucción o petición explícita o implícita de trabajo, Jev debe devolver `"decision": "extract"`. El extractor marcará `"is_task": true` y rellenará el objeto `task`.
4.  **Filtro de relevancia:** Mensajes de cortesía, confirmaciones breves ("ok", "visto", "gracias") o charlas informales deben marcarse como `"decision": "discard"`; si pasan al extractor, este debe devolver `"is_task": false`.
5.  **Nivel de prioridad:** El extractor determinará la prioridad según expresiones de urgencia del mensaje original (por ejemplo, "urgente", "para hoy", "cuando puedas").
6.  **Ambigüedad:** Si Jev no puede distinguir de forma fiable entre acción, extracción o descarte, debe devolver una confianza baja y el sistema debe retener el mensaje para revisión, sin ejecutar cambios destructivos.
7.  **Privacidad:** Jev y el LLM extractor solo recibirán contenido ofuscado. No deben intentar reconstruir, solicitar ni generar datos personales reales que no sean necesarios para la tarea.

---

## 5. Modelo de Datos y Persistencia (SQL)

### 5.1. Tabla `clients`
Almacena el directorio único de contactos/clientes para evitar duplicidad.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `name` (VARCHAR)
*   `identifier` (VARCHAR UNIQUE) - Teléfono, email o alias de Telegram
*   `created_at` (TIMESTAMP)

### 5.2. Tabla `tracked_users`
Define qué usuarios pueden ser procesados por el sistema.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `source` (ENUM: 'whatsapp', 'telegram', 'gmail')
*   `identifier` (VARCHAR) - Email, teléfono, ID o username autorizado
*   `display_name` (VARCHAR opcional)
*   `active` (BOOLEAN DEFAULT TRUE)
*   `created_at` (TIMESTAMP)
*   `updated_at` (TIMESTAMP)
*   Restricción UNIQUE sobre (`source`, `identifier`)

### 5.3. Tabla `raw_messages`
Guarda el registro auditable de todas las comunicaciones recibidas.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `client_id` (FOREIGN KEY -> `clients.id`)
*   `source` (ENUM: 'whatsapp', 'telegram', 'gmail')
*   `external_id` (VARCHAR) - ID original del mensaje en el proveedor
*   `content` (TEXT) - Texto completo recibido
*   `processed` (BOOLEAN DEFAULT FALSE)
*   `created_at` (TIMESTAMP)
*   El contenido original debe tener acceso restringido y no debe aparecer en logs ni enviarse a proveedores externos.

### 5.4. Tabla `tasks`
Guarda las tareas procesadas y validadas por el sistema.
*   `id` (UUID / Auto-increment PRIMARY KEY)
*   `client_id` (FOREIGN KEY -> `clients.id`)
*   `message_id` (FOREIGN KEY -> `raw_messages.id`)
*   `title` (VARCHAR)
*   `description` (TEXT)
*   `priority` (ENUM: 'low', 'medium', 'high')
*   `status` (ENUM: 'pending', 'in_progress', 'completed', 'discarded' DEFAULT 'pending')
*   `ai_confidence` (FLOAT)
*   `created_at` (TIMESTAMP)
*   `updated_at` (TIMESTAMP)

---

## 6. Flujo de Trabajo (End-to-End)

1.  **Captura:** Un *Worker* de Go recibe un evento de Gmail, WhatsApp o Telegram.
2.  **Filtro de Usuario:** Comprueba el canal y el identificador contra `tracked_users`. Si no existe o está inactivo, excluye el mensaje y detiene el flujo.
3.  **Normalización:** Transforma únicamente los eventos autorizados a `IncomingMessage`.
4.  **Identificación de Cliente:** Busca en la tabla `clients` por `ClientIdentifier`. Si no existe, crea el registro automáticamente.
5.  **Registro Protegido:** Inserta el mensaje original autorizado en `raw_messages` con acceso restringido.
6.  **Pre-filtro Ligero:** Si el mensaje tiene menos de 3 palabras (configurable), puede evitarse la llamada a Jev y clasificarse como descartable, salvo que el canal o el contexto indiquen que forma parte de una acción pendiente.
7.  **Ofuscación:** Detecta y reemplaza los datos personales para crear la representación destinada a IA.
8.  **Construcción de Contexto:** Crea el contexto con datos mínimos y contenido ofuscado, sin modificar el original almacenado internamente.
9.  **Clasificación:** Envía únicamente el contexto ofuscado a Jev y valida su respuesta contra el esquema permitido (`db_action`, `extract` o `discard`).
10. **Acción de Base de Datos:** Si Jev devuelve `"db_action"`, valida permisos, entidad, operación y campos; después ejecuta la modificación mediante una transacción parametrizada.
11. **Extracción (IA):** Si Jev devuelve `"extract"`, envía la representación ofuscada al LLM extractor con el JSON Schema.
12. **Persistencia de Tarea:** Si el extractor devuelve `"is_task": true`, inserta un nuevo registro en `tasks` en estado `pending`.
13. **Descarte:** Si Jev devuelve `"discard"`, marca el mensaje como procesado sin crear ni modificar tareas.
14. **Consumo Exterior:** Un frontend o API externa podrá consultar la tabla `tasks` para listar, filtrar, aprobar o modificar las tareas (CRUD).

---

## 7. Infraestructura y Despliegue

El proyecto se empaqueta mediante **Docker Compose**:

*   **Servicio `core-engine`:** Binario compilado en Go con los listeners activos y el cliente de IA.
*   **Servicio `db`:** Instancia de PostgreSQL o MariaDB.
*   **Servicio `proxy`:** Traefik o Caddy para SSL y enrutamiento (preparado para cuando se integre el dashboard/API
*   **Variables de entorno :** .env 