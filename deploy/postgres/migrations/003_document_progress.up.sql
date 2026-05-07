-- Add progress tracking to documents table.
-- Idempotent: safe to re-run on DBs that already have this column.

ALTER TABLE documents ADD COLUMN IF NOT EXISTS progress_percentage INT NOT NULL DEFAULT 0;
