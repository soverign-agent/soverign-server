-- Step-level progress tracking for long-running audit workflows.
-- Idempotent: safe to re-run on DBs that already have these columns.

alter table audit_jobs
    add column if not exists current_step  text,
    add column if not exists error_message text;

update audit_jobs
set    current_step = 'unknown'
where  current_step is null;
