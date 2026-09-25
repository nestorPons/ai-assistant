-- 003_task_subject.sql
-- Tema (subject) y fecha límite (due_date) de la tarea. El tema es obligatorio
-- en toda petición de trabajo; la fecha es opcional (NULL si no se indica).

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS subject  VARCHAR(255) NOT NULL DEFAULT '' AFTER description,
    ADD COLUMN IF NOT EXISTS due_date DATE         NULL             AFTER estimated_hours;
