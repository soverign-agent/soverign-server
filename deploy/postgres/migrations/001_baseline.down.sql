-- Rollback: drops every table created by 001_baseline.up.sql in reverse FK order.
-- DESTRUCTIVE — only for dev resets or clean-slate recreation.

-- Drop triggers first
DROP TRIGGER IF EXISTS trigger_audit_jobs_set_updated_at ON audit_jobs;
DROP TRIGGER IF EXISTS trigger_approval_requests_set_updated_at ON approval_requests;
DROP TRIGGER IF EXISTS trigger_documents_set_updated_at ON documents;
DROP TRIGGER IF EXISTS trigger_generated_documents_set_updated_at ON generated_documents;

-- Drop policies
drop policy if exists tenant_isolation on tenants;
drop policy if exists tenant_isolation on users;
drop policy if exists tenant_isolation on ai_systems;
drop policy if exists tenant_isolation on repositories;
drop policy if exists tenant_isolation on repository_scan_results;
drop policy if exists tenant_isolation on documents;
drop policy if exists tenant_isolation on embeddings;
drop policy if exists tenant_isolation on audit_jobs;
drop policy if exists tenant_isolation on audit_findings;
drop policy if exists tenant_isolation on generated_documents;
drop policy if exists tenant_isolation on document_versions;
drop policy if exists tenant_isolation on document_export_jobs;
drop policy if exists tenant_isolation on approval_requests;
drop policy if exists tenant_isolation on compliance_policies;
drop policy if exists tenant_isolation on notification_events;
drop policy if exists tenant_isolation on refresh_tokens;
drop policy if exists tenant_isolation on password_reset_tokens;

-- Disable rls (clean rollback)
alter table tenants disable row level security;
alter table users disable row level security;
alter table ai_systems disable row level security;
alter table repositories disable row level security;
alter table repository_scan_results disable row level security;
alter table documents disable row level security;
alter table embeddings disable row level security;
alter table audit_jobs disable row level security;
alter table audit_findings disable row level security;
alter table generated_documents disable row level security;
alter table document_versions disable row level security;
alter table document_export_jobs disable row level security;
alter table approval_requests disable row level security;
alter table compliance_policies disable row level security;
alter table notification_events disable row level security;
alter table refresh_tokens disable row level security;
alter table password_reset_tokens disable row level security;

-- Drop helper functions
drop function if exists current_tenant_id();
drop function if exists set_updated_at();

-- Drop tables (reverse dependency order)
drop table if exists password_reset_tokens;
drop table if exists refresh_tokens;
drop table if exists notification_events;
drop table if exists compliance_policies;
drop table if exists approval_requests;
drop table if exists document_export_jobs;
drop table if exists document_versions;
drop table if exists generated_documents;
drop table if exists audit_findings;
drop table if exists audit_jobs;
drop table if exists embeddings;
drop table if exists documents;
drop table if exists repository_scan_results;
drop table if exists repositories;
drop table if exists ai_systems;
drop table if exists users;
drop table if exists tenants;
