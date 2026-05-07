-- Add missing notification event columns
ALTER TABLE notification_events
ADD COLUMN IF NOT EXISTS priority VARCHAR(50) DEFAULT 'NORMAL',
ADD COLUMN IF NOT EXISTS action_url VARCHAR(500),
ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;
