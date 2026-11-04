ALTER TABLE task_defs DROP CONSTRAINT task_defs_kind_check;
ALTER TABLE task_defs ADD CONSTRAINT task_defs_kind_check CHECK (kind IN ('daily', 'weekly', 'monthly'));
