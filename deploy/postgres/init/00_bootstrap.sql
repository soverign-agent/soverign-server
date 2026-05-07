-- Bootstrap script for fresh PostgreSQL databases.
-- Mounted at /docker-entrypoint-initdb.d/00_bootstrap.sql via docker-compose.
-- All referenced files live under /sql (the parent ./postgres directory is mounted at /sql:ro).

\set ON_ERROR_STOP on

-- Baseline schema (latest canonical state)
\i /sql/migrations/001_baseline.up.sql

-- Post-baseline migrations (idempotent: no-op on fresh DBs, applies columns/tables if missing on legacy DBs)
\i /sql/migrations/002_audit_progress.up.sql
\i /sql/migrations/003_document_progress.up.sql
\i /sql/migrations/004_documents_ai_system_id.up.sql
\i /sql/migrations/005_notification_events_columns.up.sql
\i /sql/migrations/006_chat_sessions_messages.up.sql

-- Development seed data
\i /sql/seed/001_demo_tenant.sql
