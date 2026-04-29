-- Add progress tracking to documents table
ALTER TABLE documents ADD COLUMN IF NOT EXISTS progress_percentage INT NOT NULL DEFAULT 0;
