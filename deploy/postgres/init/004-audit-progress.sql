-- Step-level progress tracking for long-running audit workflows.
-- Adds:
--   * current_step  - canonical workflow step name (e.g. "run_static_analysis")
--   * error_message - persisted failure reason so the UI can surface it after a refresh
--
-- The audit_jobs RLS policy in 002-init-schema.sql is tenant-scoped at the row
-- level, so new columns inherit isolation automatically.
--
-- For existing dev databases the init scripts will not re-run; apply this file
-- manually with:
--   psql -f deploy/postgres/init/004-audit-progress.sql
ALTER TABLE audit_jobs
    ADD COLUMN IF NOT EXISTS current_step  TEXT,
    ADD COLUMN IF NOT EXISTS error_message TEXT;

UPDATE audit_jobs
SET    current_step = 'unknown'
WHERE  current_step IS NULL;
