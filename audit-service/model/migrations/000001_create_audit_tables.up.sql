-- Audit jobs table stores audit lifecycle
CREATE TABLE IF NOT EXISTS audit_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    audit_type TEXT NOT NULL, -- 'full' or 'incremental'
    status TEXT NOT NULL DEFAULT 'pending', -- pending, running, paused, completed, failed, cancelled
    risk_score INTEGER NOT NULL DEFAULT 0,
    risk_severity TEXT NOT NULL DEFAULT 'low', -- low, medium, high, critical
    progress_percentage INTEGER NOT NULL DEFAULT 0,
    findings_count INTEGER NOT NULL DEFAULT 0,
    critical_findings INTEGER NOT NULL DEFAULT 0,
    high_findings INTEGER NOT NULL DEFAULT 0,
    medium_findings INTEGER NOT NULL DEFAULT 0,
    low_findings INTEGER NOT NULL DEFAULT 0,
    workflow_id TEXT, -- Temporal workflow ID
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_audit_jobs_tenant_id ON audit_jobs(tenant_id);
CREATE INDEX idx_audit_jobs_repository_id ON audit_jobs(repository_id);
CREATE INDEX idx_audit_jobs_status ON audit_jobs(status);
CREATE INDEX idx_audit_jobs_created_at ON audit_jobs(created_at);
CREATE INDEX idx_audit_jobs_risk_severity ON audit_jobs(risk_severity);

-- Enable Row Level Security
ALTER TABLE audit_jobs ENABLE ROW LEVEL SECURITY;

-- RLS policy: Users can only access their tenant's audit jobs
CREATE POLICY tenant_isolation ON audit_jobs
    USING (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- Audit findings table stores individual issues found during audit
CREATE TABLE IF NOT EXISTS audit_findings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    audit_job_id UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    line_number INTEGER,
    issue_type TEXT NOT NULL, -- EU AI Act category: data_privacy, transparency, human_oversight, accuracy, security, record_keeping
    severity TEXT NOT NULL, -- critical, high, medium, low
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    remediation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_audit_findings_tenant_id ON audit_findings(tenant_id);
CREATE INDEX idx_audit_findings_audit_job_id ON audit_findings(audit_job_id);
CREATE INDEX idx_audit_findings_issue_type ON audit_findings(issue_type);
CREATE INDEX idx_audit_findings_severity ON audit_findings(severity);

-- Enable Row Level Security
ALTER TABLE audit_findings ENABLE ROW LEVEL SECURITY;

-- RLS policy: Users can only access their tenant's findings
CREATE POLICY tenant_isolation ON audit_findings
    USING (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

-- Add trigger to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_audit_jobs_set_updated_at
    BEFORE UPDATE ON audit_jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
