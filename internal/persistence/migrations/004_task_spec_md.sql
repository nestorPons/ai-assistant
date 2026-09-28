-- 004_task_spec_md.sql
-- Documento spec.md de la tarea. Se renderiza de forma determinista desde los
-- datos extraídos al crear la tarea y queda editable desde el dashboard.

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS spec_md MEDIUMTEXT NULL AFTER specifications;
