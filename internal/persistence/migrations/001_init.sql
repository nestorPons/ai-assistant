-- 001_init.sql
-- Esquema base para MariaDB del AI Task Orchestrator.

CREATE TABLE IF NOT EXISTS clients (
    id          CHAR(36)     NOT NULL,
    source      ENUM('whatsapp','telegram','gmail') NOT NULL,
    identifier  VARCHAR(255) NOT NULL,
    name        VARCHAR(255) NOT NULL DEFAULT '',
    tracked     BOOLEAN      NOT NULL DEFAULT FALSE,
    active      BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_clients_source_identifier (source, identifier)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS raw_messages (
    id          CHAR(36)     NOT NULL,
    client_id   CHAR(36)     NOT NULL,
    source      ENUM('whatsapp','telegram','gmail') NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    content     TEXT         NOT NULL,
    processed   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_raw_messages_source_external (source, external_id),
    KEY idx_raw_messages_client (client_id),
    CONSTRAINT fk_raw_messages_client FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tasks (
    id               CHAR(36)     NOT NULL,
    client_id        CHAR(36)     NOT NULL,
    message_id       CHAR(36)     NOT NULL,
    title            VARCHAR(500) NOT NULL,
    description      TEXT         NOT NULL,
    priority         ENUM('low','medium','high') NOT NULL DEFAULT 'medium',
    estimated_hours  DECIMAL(6,2) NULL,
    specifications   JSON         NULL,
    status           ENUM('pending','in_progress','completed','discarded','needs_review') NOT NULL DEFAULT 'pending',
    ai_confidence    FLOAT        NOT NULL DEFAULT 0,
    created_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_tasks_client (client_id),
    KEY idx_tasks_message (message_id),
    KEY idx_tasks_status (status),
    CONSTRAINT fk_tasks_client FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE,
    CONSTRAINT fk_tasks_message FOREIGN KEY (message_id) REFERENCES raw_messages(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
