-- Seed data for development/testing
-- Creates a default tenant and admin user for initial platform access.
-- Run this after 002-init-schema.sql.

-- =============================================================================
-- 1. Default Tenant
-- =============================================================================
INSERT INTO tenants (id, name, slug, domain, settings, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Sovereign AI Demo',
    'sovereign-demo',
    'sovereign.ai',
    '{"industry":"technology","region":"EU"}',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 2. Admin User
-- =============================================================================
-- Password: SovereignAdmin123!
-- bcrypt cost 12 hash generated via auth-service password package
INSERT INTO users (id, tenant_id, email, password_hash, role, is_active, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000001',
    'admin@sovereign.ai',
    '$2a$12$FxgAQBYwu/KJYyI5lt8t7eYxJ5f9gE86YYf/q7piSF3gkSdObT8ZO',
    'admin',
    TRUE,
    NOW(),
    NOW()
)
ON CONFLICT (tenant_id, email) DO NOTHING;
