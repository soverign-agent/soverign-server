-- Drop triggers
DROP TRIGGER IF EXISTS update_generated_documents_updated_at ON generated_documents;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop policies
DROP POLICY IF EXISTS tenant_isolation ON generated_documents;
DROP POLICY IF EXISTS tenant_isolation ON document_versions;
DROP POLICY IF EXISTS tenant_isolation ON document_export_jobs;

-- Disable RLS (for clean rollback)
ALTER TABLE generated_documents DISABLE ROW LEVEL SECURITY;
ALTER TABLE document_versions DISABLE ROW LEVEL SECURITY;
ALTER TABLE document_export_jobs DISABLE ROW LEVEL SECURITY;

-- Drop tables (order matters due to foreign keys)
DROP TABLE IF EXISTS document_export_jobs;
DROP TABLE IF EXISTS document_versions;
DROP TABLE IF EXISTS generated_documents;
