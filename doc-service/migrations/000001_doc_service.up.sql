-- Generated documents table
CREATE TABLE IF NOT EXISTS generated_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    ai_system_id UUID,
    doc_type VARCHAR(100) NOT NULL,
    title VARCHAR(500) NOT NULL,
    content JSONB NOT NULL DEFAULT '{}',
    version INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'generating',
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Document versions table for version history
CREATE TABLE IF NOT EXISTS document_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    document_id UUID NOT NULL REFERENCES generated_documents(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    content JSONB NOT NULL DEFAULT '{}',
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    change_summary TEXT,
    UNIQUE(tenant_id, document_id, version_number)
);

-- Export jobs table for async export operations
CREATE TABLE IF NOT EXISTS document_export_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    document_id UUID NOT NULL REFERENCES generated_documents(id) ON DELETE CASCADE,
    format VARCHAR(20) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    file_path TEXT,
    file_size BIGINT,
    error_message TEXT,
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Row Level Security
ALTER TABLE generated_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE generated_documents FORCE ROW LEVEL SECURITY;

ALTER TABLE document_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_versions FORCE ROW LEVEL SECURITY;

ALTER TABLE document_export_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_export_jobs FORCE ROW LEVEL SECURITY;

-- RLS Policies using current_setting for app.current_tenant
CREATE POLICY tenant_isolation ON generated_documents
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true)::UUID)
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::UUID);

CREATE POLICY tenant_isolation ON document_versions
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true)::UUID)
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::UUID);

CREATE POLICY tenant_isolation ON document_export_jobs
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant', true)::UUID)
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::UUID);

-- Updated at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger to generated_documents
CREATE TRIGGER update_generated_documents_updated_at
    BEFORE UPDATE ON generated_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Indexes
CREATE INDEX IF NOT EXISTS idx_generated_docs_tenant_ai ON generated_documents(tenant_id, ai_system_id);
CREATE INDEX IF NOT EXISTS idx_generated_docs_tenant_status ON generated_documents(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_generated_docs_tenant_type ON generated_documents(tenant_id, doc_type);
CREATE INDEX IF NOT EXISTS idx_doc_versions_tenant_doc ON document_versions(tenant_id, document_id);
CREATE INDEX IF NOT EXISTS idx_doc_versions_doc_number ON document_versions(document_id, version_number);
CREATE INDEX IF NOT EXISTS idx_export_jobs_tenant_doc ON document_export_jobs(tenant_id, document_id);
CREATE INDEX IF NOT EXISTS idx_export_jobs_status ON document_export_jobs(status);
