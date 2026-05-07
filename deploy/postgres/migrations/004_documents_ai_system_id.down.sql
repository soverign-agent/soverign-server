DROP INDEX IF EXISTS idx_documents_ai_system_id;

ALTER TABLE documents DROP COLUMN IF EXISTS ai_system_id;
