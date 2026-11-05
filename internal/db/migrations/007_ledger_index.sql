CREATE INDEX IF NOT EXISTS coin_ledger_created_idx ON coin_ledger USING btree (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS task_progress_task_period_idx ON task_progress USING btree (task_code, period);
