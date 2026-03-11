# CLAUDE.md — Go Backend Project Guide

This file is the authoritative reference for how this project is built and how Claude should assist with it.
It is split into three sections:
1. **Technology Stack** — what tools are used and why
2. **Architecture & Best Practices** — how code is structured and what conventions apply
3. **This Project** — the specific use case, domain model, and endpoints

---

## 1. Technology Stack

### Language
- **Go 1.24.x** — use the latest 1.24 patch release
- Enable `toolchain` directive in `go.mod` to pin the toolchain version

### HTTP
- **`net/http`** — stdlib foundation; use `http.Handler` and `http.HandlerFunc` throughout
- **`github.com/go-chi/chi/v5`** — routing and middleware composition
- Middleware chain order (outermost → innermost):
  `Recoverer → RequestID → StructuredLogger → Tracing → Auth → RateLimiter → Handler`

### Configuration
- Environment variables are the primary source of config
- Use **`github.com/knadh/koanf/v2`** when layered config (env → file → defaults) is needed, or a small internal `config` package for simpler services
- Validate all config at startup; fail fast with a clear error message if required values are missing
- Never read `os.Getenv` inside handlers or business logic — always pass config as a struct

### Logging
- **`log/slog`** — standard structured logger
- Use `slog.JSONHandler` in production, `slog.TextHandler` locally (driven by `LOG_FORMAT` env var)
- Inject a logger enriched with `request_id` and `trace_id` into `context.Context` at the request boundary
- Never pass a logger as a function argument — always read it from context via a small `logger.FromContext(ctx)` helper

### Metrics
- **`github.com/prometheus/client_golang`**
- Expose `/metrics` endpoint (Prometheus scrape target)
- Required metrics for every service:
  - `http_requests_total` — counter, labels: `method`, `path`, `status`
  - `http_request_duration_seconds` — histogram, labels: `method`, `path`
  - `http_requests_in_flight` — gauge
  - `db_query_duration_seconds` — histogram, labels: `query_name`
  - `build_info` — gauge with labels: `version`, `commit`, `build_time`
- Add domain-specific metrics (queue depth, retry count, etc.) as needed per project

### Tracing
- **`go.opentelemetry.io/otel`** — OpenTelemetry Go SDK (traces)
- Use `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` to instrument chi handlers automatically
- Propagate `context.Context` through every DB call, outbound HTTP call, and queue operation
- Export via OTLP; backend (Jaeger, Tempo, etc.) is deployment-specific

### Database
- **PostgreSQL** in production — driver: `github.com/jackc/pgx/v5` via `pgxpool`
- **SQLite** in local development — driver: `github.com/mattn/go-sqlite3` (CGo) or `modernc.org/sqlite` (pure Go, preferred)
- Use **`database/sql`** as the abstraction layer so repositories are driver-agnostic
- Migrations: **`github.com/pressly/goose/v3`** with SQL-only migration files in `db/migrations/`
- Write SQL migrations that are compatible with both SQLite and Postgres where possible; use build tags or separate migration dirs when they diverge

### Authentication
- **End-user auth**: OIDC/OAuth2 — use `github.com/coreos/go-oidc/v3`
- **Service-to-service auth**: JWT with asymmetric signing (ES256 preferred)
  - Always validate signatures server-side on every request
  - Enforce short expiry (≤15 minutes) for service tokens
- Authorization logic lives in the service layer, not the handler

### API Documentation
- **Swagger/OpenAPI** — annotate handlers with `swaggo/swag` comments; generate spec via `swag init`
- Serve the Swagger UI at `/docs`

---

## 2. Architecture & Best Practices

### Layered Architecture

The dependency chain flows in one direction only:

```
Handler → Service → Repository
```

- **Repository** — data access only; speaks SQL; returns domain types
- **Service** — business logic; orchestrates one or more repositories; owns transactions
- **Handler** — HTTP concerns only; decodes requests, calls service, encodes responses; no SQL, no business rules

Each layer depends only on interfaces defined by the layer below it, never on concrete types from another package.

```
┌─────────────┐
│   Handler   │  HTTP in/out, request decoding, response encoding
└──────┬──────┘
       │ calls
┌──────▼──────┐
│   Service   │  business logic, validation, authorization checks
└──────┬──────┘
       │ calls
┌──────▼──────┐
│ Repository  │  SQL queries, DB connection, no business logic
└─────────────┘
```

### Project Layout

Keep it flat. Avoid deeply nested packages. Every package name should be a single, obvious noun.

```
.
├── CLAUDE.md
├── README.md
├── go.mod
├── go.sum
├── .env.example
├── Makefile
│
├── main.go                  # entry point: wire everything together, start server
│
├── config/
│   └── config.go            # Config struct, Load() function, validation
│
├── db/
│   ├── db.go                # open connection, run migrations on startup
│   └── migrations/
│       ├── 001_init.sql
│       └── 002_add_xyz.sql
│
├── domain/                  # pure Go types; no imports from other internal packages
│   └── todo.go              # e.g. type Todo struct{...}, type TodoStatus string, etc.
│
├── repository/
│   ├── repository.go        # Repository interface(s)
│   └── todo.go              # SQL implementation
│
├── service/
│   ├── service.go           # Service interface(s)
│   └── todo.go              # business logic implementation
│
├── handler/
│   ├── handler.go           # shared handler setup, helper functions (respond, decode)
│   ├── middleware.go        # request ID, logger injection, auth middleware
│   ├── routes.go            # all route registrations in one place
│   └── todo.go              # todo HTTP handlers
│
├── docs/                    # generated by swag init — do not edit manually
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
│
└── internal/
    ├── logger/              # FromContext, WithContext helpers
    └── apierror/            # typed API errors → HTTP status mapping
```

**Rules:**
- `domain/` has zero internal imports — it is the shared vocabulary
- `repository/` imports `domain/` only
- `service/` imports `domain/` and `repository/` interfaces only
- `handler/` imports `domain/` and `service/` interfaces only
- `main.go` is the only place that imports everything and wires it together

### Interfaces

Define interfaces in the consuming package, not the implementing package.

```go
// service/todo.go defines what it needs from the repository
type TodoRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Todo, error)
    List(ctx context.Context, filter domain.TodoFilter) ([]domain.Todo, error)
    Create(ctx context.Context, todo *domain.Todo) error
    Update(ctx context.Context, todo *domain.Todo) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Error Handling

- Always wrap errors with context: `fmt.Errorf("getting todo %s: %w", id, err)`
- Define typed errors in `internal/apierror/` that carry an HTTP status code and a user-safe message
- Handlers check for `apierror` types and respond accordingly; unknown errors → 500
- Never leak internal error messages or stack traces to API consumers

```go
// internal/apierror/apierror.go
type APIError struct {
    Code    int
    Message string
}
func (e *APIError) Error() string { return e.Message }

var (
    ErrNotFound   = &APIError{Code: 404, Message: "resource not found"}
    ErrBadRequest = func(msg string) *APIError { return &APIError{Code: 400, Message: msg} }
)
```

### Context Propagation

- Every function that touches I/O (DB, HTTP, queue) must accept `ctx context.Context` as its first argument
- Never store context in a struct
- Always check `ctx.Err()` in long-running loops

### HTTP Conventions

- All API endpoints are prefixed with `/v1`
- System endpoints at root level: `/health`, `/ready`, `/metrics`, `/docs`
- Request and response bodies are JSON
- Use a shared `respond(w, status, payload)` helper in `handler/handler.go`
- Use a shared `decode(r, &target)` helper that validates Content-Type and returns an `apierror`
- Return `{"error": "message"}` for all error responses — never a bare string

### Graceful Shutdown

Always implement graceful shutdown in `main.go`:

```go
srv := &http.Server{Addr: cfg.Addr, Handler: router}

go func() { srv.ListenAndServe() }()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
srv.Shutdown(ctx)
```

### Testing

- Unit tests: `_test.go` files alongside source; use `github.com/stretchr/testify`
- Integration tests: use `github.com/testcontainers/testcontainers-go` for real Postgres
- Local dev uses SQLite for fast tests without Docker
- Test file naming: `todo_test.go` next to `todo.go`
- Interfaces make all layers independently testable with simple fakes — no mocking frameworks

### Makefile Targets

Every project must have at minimum:

```makefile
run          # go run ./main.go
build        # go build -o bin/server ./main.go
test         # go test ./...
lint         # golangci-lint run
migrate      # goose up
migrate-down # goose down
swagger      # swag init
```

---

## 3. This Project — Todo API

### Overview

A simple REST API for managing todo items. Demonstrates the full stack end-to-end.

### Domain Model

```go
// domain/todo.go
type TodoStatus string

const (
    TodoStatusOpen    TodoStatus = "open"
    TodoStatusDone    TodoStatus = "done"
)

type Todo struct {
    ID          uuid.UUID  `json:"id"`
    Title       string     `json:"title"`
    Description string     `json:"description"`
    Status      TodoStatus `json:"status"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

### Endpoints

| Method   | Path              | Description              | Auth     |
|----------|-------------------|--------------------------|----------|
| `GET`    | `/health`         | Liveness probe           | None     |
| `GET`    | `/ready`          | Readiness probe (DB ping)| None     |
| `GET`    | `/metrics`        | Prometheus metrics       | None     |
| `GET`    | `/docs`           | Swagger UI               | None     |
| `GET`    | `/v1/todos`       | List all todos           | JWT      |
| `POST`   | `/v1/todos`       | Create a todo            | JWT      |
| `GET`    | `/v1/todos/{id}`  | Get a single todo        | JWT      |
| `PUT`    | `/v1/todos/{id}`  | Update a todo            | JWT      |
| `DELETE` | `/v1/todos/{id}`  | Delete a todo            | JWT      |

### Database Schema

```sql
-- db/migrations/001_init.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS todos (
    id          TEXT        PRIMARY KEY,  -- UUID as text (compatible with SQLite + Postgres)
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'open',
    created_at  TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS todos;
```

### Environment Variables

```bash
# .env.example

# Server
PORT=8080
LOG_FORMAT=text           # text | json
ENV=development           # development | production

# Database
DB_DRIVER=sqlite          # sqlite | postgres
DATABASE_URL=./dev.db     # for sqlite: file path; for postgres: full DSN

# Auth
JWT_PUBLIC_KEY_PATH=./keys/public.pem

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
```

### Local Development Setup

```bash
cp .env.example .env
make migrate        # runs goose up with SQLite
make swagger        # generates docs/
make run
```

### Production Setup

```bash
DB_DRIVER=postgres \
DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=require" \
LOG_FORMAT=json \
ENV=production \
make run
```

### Key Implementation Notes

- The `repository` layer detects `DB_DRIVER` at startup and selects the appropriate SQL dialect where needed (e.g. `RETURNING` clauses are Postgres-only — use `LastInsertId` for SQLite)
- UUIDs are stored as `TEXT` in SQLite and `UUID` type in Postgres; the repository handles this transparently
- All timestamps are stored and returned as UTC
- The `/ready` endpoint performs a `SELECT 1` against the DB; if it fails, respond `503`
- The `/health` endpoint always returns `200` as long as the process is running
