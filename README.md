# Sovereign AI Backend Services

This repository contains all Go backend microservices for the Sovereign AI Compliance Orchestration Platform.

## Architecture

```
                    ┌─────────────────┐
                    │   api-gateway   │  ← HTTP :8080 (sole entry point)
                    │  grpc-gateway   │
                    └────────┬────────┘
                             │ gRPC
         ┌───────────────────┼───────────────────┐
         │                   │                   │
    ┌────┴────┐        ┌────┴────┐        ┌────┴────┐
    │  auth   │        │   org   │        │  repo   │
    │ :9081   │        │ :9085   │        │ :9083   │
    └────┬────┘        └────┬────┘        └────┬────┘
         │                   │                   │
    ┌────┴────┐        ┌────┴────┐        ┌────┴────┐
    │   rag   │        │  audit  │        │   doc   │
    │ :9082   │        │ :9086   │        │ :9888   │
    └────┬────┘        └────┬────┘        └────-────┘
         │                   │                   
    ┌────┴────┐        ┌────┴────┐       
    │   notif │        │  agent  │
    │ :9087   │        │ :9088   │
    └─────────┘        └─────────┘
```

## Services

| Service | gRPC Port | Protocol | Description |
|---------|-----------|----------|-------------|
| `api-gateway` | — | HTTP + gRPC gateway | Unified entry point with JWT, rate limiting, circuit breaking |
| `auth-service` | 9081 | gRPC | Authentication and RBAC authorization |
| `org-service` | 9085 | gRPC | Tenant and organization management |
| `repo-service` | 9083 | gRPC | Code repository integration and static analysis |
| `rag-service` | 9082 | gRPC | Document processing, embeddings, and vector search |
| `audit-service` | 9086 | gRPC | Compliance audit job management and risk scoring |
| `doc-service` | 9888 | gRPC | Compliance document generation and version management |
| `agent-orchestrator` | 9088 | gRPC | Multi-agent orchestration with Eino ADK |
| `notification-service` | 9087 | gRPC | Multi-channel notification delivery |

All internal services communicate exclusively via gRPC. The `api-gateway` is the only service that exposes HTTP (port 8080), translating REST requests to gRPC via grpc-gateway/v2.

## Shared Libraries

- `shared/proto` — gRPC protocol buffer definitions
- `shared/config` — Configuration utilities
- `shared/tenant` — Tenant context propagation for RLS
- `shared/database` — Database connection and transaction utilities
- `shared/security` — Encryption, JWT, and security utilities
- `shared/llm` — LLM client abstraction layer
- `shared/grpcclient` — Reusable gRPC client connection helpers

## Local Development

### Prerequisites

- Go 1.25+
- PostgreSQL 16
- Redis
- Temporal
- protoc + Go plugins (for regenerating protobufs)

### Start Infrastructure

```bash
cd deploy
docker-compose up -d

# If port 3000 is already in use, run Grafana on another host port:
GRAFANA_PORT=3002 docker-compose up -d
```

### Run Services

```bash
# API Gateway (sole HTTP entry point)
go run ./api-gateway/main.go -f ./api-gateway/etc/config.yaml

# Auth Service
go run ./auth-service/main.go -f ./auth-service/etc/config.yaml

# Org Service
go run ./org-service/main.go -f ./org-service/etc/config.yaml

# ... etc for each service
```

### Regenerate Protobufs

```bash
make proto
```

### Testing

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# List all gRPC methods (requires grpcurl)
grpcurl -plaintext localhost:9081 list
grpcurl -plaintext localhost:9085 list
```
