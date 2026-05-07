# PostgreSQL Deployment

This directory contains the single source of truth for Sovereign AI's database schema, migrations, and seed data.

## Layout

```
deploy/postgres/
├── init/
│   └── 00_bootstrap.sql          # Docker entrypoint: runs once on fresh volume init
├── migrations/
│   ├── 001_baseline.up.sql       # Canonical latest schema (all tables, RLS, indexes)
│   ├── 001_baseline.down.sql     # Destructive rollback (dev only)
│   ├── 002_audit_progress.up.sql # Idempotent ALTER for legacy DBs
│   ├── 002_audit_progress.down.sql
│   ├── 003_document_progress.up.sql
│   ├── 003_document_progress.down.sql
│   ├── 004_documents_ai_system_id.up.sql
│   └── 004_documents_ai_system_id.down.sql
└── seed/
    └── 001_demo_tenant.sql       # Dev-only seed (tenant, admin, policy)
```

## How it works

- `docker-compose.yml` mounts `./postgres/init` to the container's `/docker-entrypoint-initdb.d`.
- On first volume creation, PostgreSQL executes files in that directory alphabetically.
- `00_bootstrap.sql` chains every migration and seed via `\i` includes.
- The parent `./postgres` directory is also mounted at `/sql:ro` so `\i` can reach subdirectories.

## Applying to an existing (legacy) database

If you have a database initialized from an older baseline and need to converge to the latest state, run migrations sequentially:

```bash
psql -U sovereign -d sovereign -f migrations/002_audit_progress.up.sql
psql -U sovereign -d sovereign -f migrations/003_document_progress.up.sql
psql -U sovereign -d sovereign -f migrations/004_documents_ai_system_id.up.sql
```

All migrations are idempotent (`IF NOT EXISTS` / `IF EXISTS`) so re-running is safe.

## Adding a new migration

1. Create `NNN_description.up.sql` and matching `.down.sql` in `migrations/`.
2. Append `\i /sql/migrations/NNN_description.up.sql` to `init/00_bootstrap.sql`.
3. If it adds tables/columns, consider whether the column also belongs in `001_baseline.up.sql` so fresh DBs and legacy DBs converge.

## Dev reset (fresh volume)

```bash
cd server/deploy
docker compose down -v
docker compose up -d postgres
```

The `-v` removes the named volume, forcing PostgreSQL to re-run `docker-entrypoint-initdb.d`.

## Production notes

- Do **not** run `seed/` in production (remove the `\i` line from `00_bootstrap.sql` or use a separate production bootstrap).
- Do **not** run `*.down.sql` in production without a deliberate maintenance window and backup.
