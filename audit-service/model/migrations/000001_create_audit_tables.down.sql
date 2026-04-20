DROP POLICY IF EXISTS tenant_isolation ON audit_findings;
DROP TABLE IF EXISTS audit_findings;

DROP POLICY IF EXISTS tenant_isolation ON audit_jobs;
DROP TABLE IF EXISTS audit_jobs;
