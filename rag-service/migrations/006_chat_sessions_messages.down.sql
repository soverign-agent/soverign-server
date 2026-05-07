-- Roll back chat sessions and messages migration.

DROP INDEX IF EXISTS idx_chat_messages_tenant_id;
DROP INDEX IF EXISTS idx_chat_messages_session_created;
DROP INDEX IF EXISTS idx_chat_sessions_tenant_user_updated;

DROP POLICY IF EXISTS tenant_isolation ON chat_messages;
DROP POLICY IF EXISTS tenant_isolation ON chat_sessions;

DROP TRIGGER IF EXISTS trigger_chat_sessions_set_updated_at ON chat_sessions;

DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_sessions;
