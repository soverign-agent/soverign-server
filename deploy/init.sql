-- Baseline database initialization script for Sovereign AI Compliance Platform
-- Run this against a fresh PostgreSQL 16 + pgvector instance.

-- =============================================================================
-- 1. Extensions
-- =============================================================================
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- =============================================================================
-- 2. Base Tables
-- =============================================================================

CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    domain VARCHAR(255),
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    is_active BOOLEAN DEFAULT TRUE,
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, email)
);

CREATE TABLE IF NOT EXISTS ai_systems (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    risk_classification VARCHAR(50) NOT NULL DEFAULT 'limited',
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    provider VARCHAR(50) NOT NULL,
    encrypted_credentials BYTEA,
    last_scanned_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    content TEXT,
    source VARCHAR(100),
    doc_type VARCHAR(100),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL DEFAULT 0,
    chunk_text TEXT,
    embedding VECTOR(1536),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ai_system_id UUID REFERENCES ai_systems(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'queued',
    progress INT NOT NULL DEFAULT 0,
    result JSONB DEFAULT '{}',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_findings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    audit_job_id UUID NOT NULL REFERENCES audit_jobs(id) ON DELETE CASCADE,
    severity VARCHAR(50) NOT NULL DEFAULT 'medium',
    category VARCHAR(100),
    description TEXT,
    remediation TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS generated_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ai_system_id UUID REFERENCES ai_systems(id) ON DELETE SET NULL,
    doc_type VARCHAR(100) NOT NULL,
    title VARCHAR(500),
    content JSONB DEFAULT '{}',
    version INT NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS approval_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    request_type VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID NOT NULL,
    requested_by UUID NOT NULL REFERENCES users(id),
    reviewer_id UUID REFERENCES users(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS approval_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    approval_request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL,
    actor_id UUID NOT NULL REFERENCES users(id),
    comment TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS compliance_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    policy_type VARCHAR(100) NOT NULL,
    rules JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    recipient VARCHAR(255),
    subject VARCHAR(500),
    body TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    sent_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- 3. Row Level Security
-- =============================================================================

-- Enable and force RLS on all tenant-scoped tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

ALTER TABLE ai_systems ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_systems FORCE ROW LEVEL SECURITY;

ALTER TABLE repositories ENABLE ROW LEVEL SECURITY;
ALTER TABLE repositories FORCE ROW LEVEL SECURITY;

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;

ALTER TABLE embeddings ENABLE ROW LEVEL SECURITY;
ALTER TABLE embeddings FORCE ROW LEVEL SECURITY;

ALTER TABLE audit_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_jobs FORCE ROW LEVEL SECURITY;

ALTER TABLE audit_findings ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_findings FORCE ROW LEVEL SECURITY;

ALTER TABLE generated_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE generated_documents FORCE ROW LEVEL SECURITY;

ALTER TABLE approval_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_requests FORCE ROW LEVEL SECURITY;

ALTER TABLE approval_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE approval_history FORCE ROW LEVEL SECURITY;

ALTER TABLE compliance_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_policies FORCE ROW LEVEL SECURITY;

ALTER TABLE notification_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_events FORCE ROW LEVEL SECURITY;

ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens FORCE ROW LEVEL SECURITY;

ALTER TABLE password_reset_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE password_reset_tokens FORCE ROW LEVEL SECURITY;

-- Tenants table: users can only see their own tenant
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- 4. RLS Policies
-- =============================================================================

-- Helper function to extract current tenant from session variable
CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_tenant', TRUE), '')::UUID;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Tenants policy
CREATE POLICY tenant_isolation ON tenants
    FOR ALL
    USING (id = current_tenant_id())
    WITH CHECK (id = current_tenant_id());

-- Users policy
CREATE POLICY tenant_isolation ON users
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- AI Systems policy
CREATE POLICY tenant_isolation ON ai_systems
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Repositories policy
CREATE POLICY tenant_isolation ON repositories
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Documents policy
CREATE POLICY tenant_isolation ON documents
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Embeddings policy
CREATE POLICY tenant_isolation ON embeddings
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Audit Jobs policy
CREATE POLICY tenant_isolation ON audit_jobs
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Audit Findings policy
CREATE POLICY tenant_isolation ON audit_findings
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Generated Documents policy
CREATE POLICY tenant_isolation ON generated_documents
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Approval Requests policy
CREATE POLICY tenant_isolation ON approval_requests
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Approval History policy
CREATE POLICY tenant_isolation ON approval_history
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Compliance Policies policy
CREATE POLICY tenant_isolation ON compliance_policies
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Notification Events policy
CREATE POLICY tenant_isolation ON notification_events
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Refresh Tokens policy
CREATE POLICY tenant_isolation ON refresh_tokens
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- Password Reset Tokens policy
CREATE POLICY tenant_isolation ON password_reset_tokens
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- =============================================================================
-- 5. Indexes
-- =============================================================================

-- HNSW index on embeddings for fast vector search
CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_users_tenant_email ON users(tenant_id, email);
CREATE INDEX IF NOT EXISTS idx_ai_systems_tenant_status ON ai_systems(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_repositories_tenant_provider ON repositories(tenant_id, provider);
CREATE INDEX IF NOT EXISTS idx_documents_tenant_type ON documents(tenant_id, doc_type);
CREATE INDEX IF NOT EXISTS idx_embeddings_tenant_doc ON embeddings(tenant_id, document_id);
CREATE INDEX IF NOT EXISTS idx_audit_jobs_tenant_system ON audit_jobs(tenant_id, ai_system_id);
CREATE INDEX IF NOT EXISTS idx_audit_findings_tenant_job ON audit_findings(tenant_id, audit_job_id);
CREATE INDEX IF NOT EXISTS idx_generated_docs_tenant_system ON generated_documents(tenant_id, ai_system_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_tenant_status ON approval_requests(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_approval_history_tenant_request ON approval_history(tenant_id, approval_request_id);
CREATE INDEX IF NOT EXISTS idx_compliance_policies_tenant_type ON compliance_policies(tenant_id, policy_type);
CREATE INDEX IF NOT EXISTS idx_notification_events_tenant_status ON notification_events(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_hash ON refresh_tokens(user_id, token_hash);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_hash ON password_reset_tokens(token_hash);
