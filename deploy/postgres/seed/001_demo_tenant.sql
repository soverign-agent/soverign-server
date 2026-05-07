-- Seed data for development and testing.
-- Creates a default tenant and admin user for initial platform access.

-- Default Tenant
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

-- Admin User
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

-- Default Compliance Policy
INSERT INTO compliance_policies (id, tenant_id, name, policy_type, rules, is_active, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000003',
    '00000000-0000-0000-0000-000000000001',
    'EU AI Act Default Policy',
    'eu_ai_act',
    '{"risk_levels":["unacceptable","high","limited","minimal"],"auto_scan":true,"require_human_review_for_high_risk":true}',
    TRUE,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;
