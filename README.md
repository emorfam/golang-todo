# Todo API

A REST API for managing todo items, built with Go. Demonstrates a layered architecture (handler → service → repository) with SQLite for local development and PostgreSQL for production.

## Requirements

- Go 1.24+
- [goose](https://github.com/pressly/goose) — database migrations
- [swag](https://github.com/swaggo/swag) — API doc generation (optional)
- [golangci-lint](https://golangci-lint.run) — linting (optional)
- OpenSSL — for generating JWT keys
- Docker — only for PostgreSQL integration tests

## Quick start (SQLite)

```bash
# 1. Clone and enter the project
git clone <repo-url>
cd golang-todo

# 2. Copy and edit environment config
cp .env.example .env

# 3. Generate JWT keys (see Authentication section below)
mkdir -p keys
openssl ecparam -name prime256v1 -genkey -noout -out keys/private.pem
openssl ec -in keys/private.pem -pubout -out keys/public.pem

# 4. Run database migrations
make migrate

# 5. Start the server — .env is loaded automatically
make run
```

The server starts on `http://localhost:8080`.

## Environment variables

All configuration is read from environment variables. Copy `.env.example` to `.env` and adjust as needed. The server automatically loads `.env` from the working directory at startup — no shell sourcing required. Actual environment variables always take precedence over values in the file. The server validates required values at startup and exits immediately if any are missing or invalid.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port the HTTP server listens on |
| `LOG_FORMAT` | `text` | Log format: `text` (local) or `json` (production) |
| `ENV` | `development` | Environment name: `development` or `production` |
| `DB_DRIVER` | `sqlite` | Database driver: `sqlite` or `postgres` |
| `DATABASE_URL` | `./dev.db` | SQLite file path or full PostgreSQL DSN |
| `JWT_PUBLIC_KEY_PATH` | `./keys/public.pem` | Path to the ES256 public key PEM file |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OTLP collector endpoint for distributed tracing |

If `JWT_PUBLIC_KEY_PATH` is missing or unreadable the server starts but every request to `/v1/*` returns `401`. Tracing is optional — if the OTLP endpoint is unreachable the server logs a warning and continues without tracing.

## Authentication

All `/v1/*` endpoints require a JWT in the `Authorization` header:

```
Authorization: Bearer <token>
```

The server validates tokens using the following rules:

- Algorithm must be **ES256** (ECDSA P-256). Tokens signed with any other algorithm are rejected.
- The token must be **valid** (signature check, expiry).
- The token lifetime (`exp − iat`) must not exceed **15 minutes**.

### Generating keys

The server only needs the **public key** at runtime. Keep the private key secure and never commit it.

```bash
mkdir -p keys

# Generate private key
openssl ecparam -name prime256v1 -genkey -noout -out keys/private.pem

# Derive public key
openssl ec -in keys/private.pem -pubout -out keys/public.pem
```

Set `JWT_PUBLIC_KEY_PATH=./keys/public.pem` in your `.env` file.

### Issuing tokens

Tokens are issued by whatever identity provider or script controls the private key. For local development the project includes a small token generator:

```bash
# Print a token with default options (subject: dev-user, lifetime: 10m)
go run ./cmd/tokengen

# Custom subject and lifetime
go run ./cmd/tokengen -sub alice -ttl 5m

# Use directly in a request
TOKEN=$(go run ./cmd/tokengen)
curl http://localhost:8080/v1/todos -H "Authorization: Bearer $TOKEN"
```

The generator accepts these flags:

| Flag | Default | Description |
|---|---|---|
| `-key` | `keys/private.pem` | Path to the ES256 private key PEM file |
| `-sub` | `dev-user` | JWT `sub` claim |
| `-ttl` | `10m` | Token lifetime (must be ≤ 15m) |

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | — | Liveness probe, always `200` |
| `GET` | `/ready` | — | Readiness probe, `503` if DB unreachable |
| `GET` | `/metrics` | — | Prometheus metrics |
| `GET` | `/docs` | — | Swagger UI |
| `GET` | `/v1/todos` | JWT | List todos (optional `?status=open\|done`) |
| `POST` | `/v1/todos` | JWT | Create a todo |
| `GET` | `/v1/todos/{id}` | JWT | Get a single todo |
| `PUT` | `/v1/todos/{id}` | JWT | Update a todo |
| `DELETE` | `/v1/todos/{id}` | JWT | Delete a todo |

### Example requests

```bash
TOKEN=<your-token>

# List all todos
curl http://localhost:8080/v1/todos \
  -H "Authorization: Bearer $TOKEN"

# Filter by status
curl "http://localhost:8080/v1/todos?status=open" \
  -H "Authorization: Bearer $TOKEN"

# Create a todo
curl http://localhost:8080/v1/todos \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy milk","description":"Whole milk, 2 liters"}'

# Update a todo
curl -X PUT http://localhost:8080/v1/todos/<id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy milk","description":"Whole milk, 2 liters","status":"done"}'

# Delete a todo
curl -X DELETE http://localhost:8080/v1/todos/<id> \
  -H "Authorization: Bearer $TOKEN"
```

All error responses use the shape `{"error": "message"}`.

## Makefile targets

```bash
make run              # go run ./main.go
make build            # compile to bin/server
make test             # unit tests (SQLite, no Docker)
make test-integration # integration tests (requires Docker for PostgreSQL)
make lint             # golangci-lint run
make migrate          # goose up (uses DB_DRIVER and DATABASE_URL from env)
make migrate-down     # goose down
make swagger          # regenerate docs/ from swaggo annotations
```

## Production setup (PostgreSQL)

```bash
DB_DRIVER=postgres \
DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=require" \
LOG_FORMAT=json \
ENV=production \
JWT_PUBLIC_KEY_PATH=/etc/secrets/public.pem \
make migrate

DB_DRIVER=postgres \
DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=require" \
LOG_FORMAT=json \
ENV=production \
JWT_PUBLIC_KEY_PATH=/etc/secrets/public.pem \
./bin/server
```

## Project structure

```
.
├── main.go              # Entry point: wires all layers and starts the server
├── config/              # Config struct and loader (koanf, env vars)
├── db/                  # Database connection and goose migration runner
│   └── migrations/      # SQL migration files
├── domain/              # Pure Go types (Todo, TodoStatus) — no internal imports
├── repository/          # SQL data access layer (SQLite + PostgreSQL)
├── service/             # Business logic layer
├── handler/             # HTTP handlers, middleware, and router
├── internal/
│   ├── apierror/        # Typed API errors mapped to HTTP status codes
│   └── logger/          # slog context helpers (FromContext, WithContext)
├── metrics/             # Prometheus metric definitions
└── docs/                # Generated Swagger spec (do not edit manually)
```
