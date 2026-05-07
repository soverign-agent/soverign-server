-- Migration: Chat sessions and messages for the agentic RAG chatbot.
-- Idempotent: safe to re-run on databases that already have these objects.
--
-- Tables added:
--   chat_sessions  - per-tenant, per-user conversation containers
--   chat_messages  - individual messages with role, content, citations (JSONB)
--
-- Tenant isolation is enforced via Row Level Security policies that check
-- tenant_id against current_setting('app.current_tenant'). All queries from
-- services must run inside a transaction that has SET LOCAL app.current_tenant.

-- =============================================================================
-- 1. Tables
-- =============================================================================

CREATE TABLE IF NOT EXISTS chat_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL DEFAULT 'New Chat',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content     TEXT NOT NULL,
    citations   JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- 2. Updated-at trigger on chat_sessions
-- =============================================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'trigger_chat_sessions_set_updated_at'
    ) THEN
        CREATE TRIGGER trigger_chat_sessions_set_updated_at
            BEFORE UPDATE ON chat_sessions
            FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END
$$;

-- =============================================================================
-- 3. Row Level Security
-- =============================================================================

ALTER TABLE chat_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_sessions FORCE ROW LEVEL SECURITY;

ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_messages FORCE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = current_schema()
          AND tablename  = 'chat_sessions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON chat_sessions
            FOR ALL
            USING (tenant_id = current_tenant_id())
            WITH CHECK (tenant_id = current_tenant_id());
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = current_schema()
          AND tablename  = 'chat_messages'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON chat_messages
            FOR ALL
            USING (tenant_id = current_tenant_id())
            WITH CHECK (tenant_id = current_tenant_id());
    END IF;
END
$$;

-- =============================================================================
-- 4. Indexes
-- =============================================================================

-- Listing recent sessions for a user within a tenant.
CREATE INDEX IF NOT EXISTS idx_chat_sessions_tenant_user_updated
    ON chat_sessions (tenant_id, user_id, updated_at DESC);

-- Paginating messages within a session in chronological order.
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_created
    ON chat_messages (session_id, created_at);

-- Auxiliary index for cross-session tenant scans (e.g. analytics).
CREATE INDEX IF NOT EXISTS idx_chat_messages_tenant_id
    ON chat_messages (tenant_id);
