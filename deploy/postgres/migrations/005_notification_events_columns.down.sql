-- Remove notification event columns
ALTER TABLE notification_events
DROP COLUMN IF EXISTS priority,
DROP COLUMN IF EXISTS action_url,
DROP COLUMN IF EXISTS read_at;
