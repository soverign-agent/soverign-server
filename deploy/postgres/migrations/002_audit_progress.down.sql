alter table audit_jobs
    drop column if exists current_step,
    drop column if exists error_message;
