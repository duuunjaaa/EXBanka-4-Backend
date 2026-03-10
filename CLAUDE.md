# EXBanka-4-Backend

## Project overview
Go-based microservices backend for EXBanka. Services communicate via gRPC. The API Gateway is the only HTTP-facing service.

## Go module
- Module path: `github.com/exbanka/backend`
- `go.mod` and `go.sum` at the repo root (single-module monorepo)

## Repository structure
```
services/        # One subdirectory per microservice
shared/          # Protobuf definitions and generated Go bindings
config/          # Environment-specific configuration (placeholder)
deploy/          # Kubernetes / Helm / Docker manifests (placeholder)
scripts/         # Dev and ops scripts
docs/            # Architecture docs, runbooks (placeholder)
```

## Services

| Service | Port | Protocol | Database |
|---|---|---|---|
| `employee-service` | 50051 | gRPC | PostgreSQL 16 on 5433 |
| `auth-service` | 50052 | gRPC | none (uses employee-service via gRPC) |
| `api-gateway` | 8081 | HTTP (Gin) | none |

## Service layout conventions
```
services/<name>/
  db/                  # SQL schema (not all services have a DB)
  handlers/            # gRPC or HTTP handler implementations
  models/              # Data structs (where needed)
  docker-compose.yml   # PostgreSQL container (only DB-backed services)
  main.go              # Entry point
```

## Shared Protobuf
- Source definitions: `shared/proto/*.proto`
- Generated Go bindings (committed): `shared/pb/<service>/`
- After editing a `.proto` file, regenerate with:
```bash
export PATH="$PATH:$(go env GOPATH)/bin"
protoc --go_out=shared/pb/<service> --go_opt=paths=source_relative \
       --go-grpc_out=shared/pb/<service> --go-grpc_opt=paths=source_relative \
       -I shared/proto shared/proto/<service>.proto
```
- The generated `*.pb.go` files have a `DO NOT EDIT` comment — this is expected; always regenerate via protoc, never hand-edit them.

## Database
- Database-per-service: every DB-backed service has its own PostgreSQL via Docker Compose.
- Schema in `db/schema.sql`, auto-applied on first container startup via `/docker-entrypoint-initdb.d/`.
- No `CREATE DATABASE` needed in SQL — handled by `POSTGRES_DB` env var.

## Running the full stack
```bash
./scripts/dev.sh
```
This starts the employee database container (waits for readiness), then all three Go services. Ctrl+C stops the Go processes; the database container keeps running.

To start only the database for a service:
```bash
cd services/<service-name>
docker compose up -d
```

## API Gateway endpoints
All employee routes require `Authorization: Bearer <access_token>` with the `ADMIN` role.

| Method | Path | Auth |
|---|---|---|
| POST | `/login` | none |
| POST | `/refresh` | none |
| GET | `/employees` | ADMIN |
| GET | `/employees/search?email=&ime=&prezime=&pozicija=` | ADMIN |
| POST | `/employees` | ADMIN |

## Auth
- JWT signed with HMAC-SHA256. Access tokens expire in 15 min, refresh tokens in 7 days.
- The JWT secret is currently hardcoded in `services/auth-service/handlers/grpc_server.go` and `services/api-gateway/middleware/auth.go` — move to environment variable before production.
- `ADMIN` role bypasses all other role checks.
- New employees are created with `aktivan=false` and no password; they cannot log in until activated.
