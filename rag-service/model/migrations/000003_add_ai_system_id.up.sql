-- Add ai_system_id to documents table for system-scoped uploads
ALTER TABLE documents ADD COLUMN IF NOT EXISTS ai_system_id UUID;

-- Create index for filtering documents by AI system
CREATE INDEX IF NOT EXISTS idx_documents_ai_system_id ON documents(ai_system_id);
