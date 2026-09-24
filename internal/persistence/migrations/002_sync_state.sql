-- 002_sync_state.sql
-- Cursor de sincronización incremental por canal (Gmail historyId, etc.).

CREATE TABLE IF NOT EXISTS sync_state (
    source       ENUM('whatsapp','telegram','gmail') NOT NULL,
    cursor_value VARCHAR(255) NOT NULL DEFAULT '',
    expires_at   TIMESTAMP    NULL DEFAULT NULL,
    updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (source)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
