-- 005_task_drop_estimated_hours.sql
-- Se elimina estimated_hours: no es un dato necesario del flujo de tareas.

ALTER TABLE tasks
    DROP COLUMN IF EXISTS estimated_hours;
