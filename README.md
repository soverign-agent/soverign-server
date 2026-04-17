# Sovereign AI Backend Services

This repository contains all Go backend microservices for the Sovereign AI Compliance Orchestration Platform.

## Services

- `api-gateway` - Unified API gateway with JWT validation, rate limiting, and circuit breaking
- `auth-service` - Authentication and RBAC authorization
- `org-service` - Tenant and organization management
- `repo-service` - Code repository integration and static analysis
- `rag-service` - Document processing, embeddings, and vector search
- `audit-service` - Compliance audit job management and risk scoring
- `doc-service` - Compliance document generation and version management
- `agent-orchestrator` - Multi-agent orchestration with Eino ADK
- `notification-service` - Multi-channel notification delivery

## Shared Libraries

- `shared/proto` - gRPC protocol buffer definitions
- `shared/config` - Configuration utilities
- `shared/tenant` - Tenant context propagation for RLS
- `shared/database` - Database connection and transaction utilities
- `shared/security` - Encryption, JWT, and security utilities
- `shared/llm` - LLM client abstraction layer
- `shared/agent` - Agent base types and A2A protocol

## Local Development

```bash
# Start all dependencies
cd deploy
docker-compose up -d

# If port 3000 is already in use, run Grafana on another host port:
GRAFANA_PORT=3002 docker-compose up -d

# Run service
go run ./api-gateway/cmd/main.go
```
