# TaskFlow API

A production-grade REST API for a minimal task management system - users, projects, and tasks with JWT auth, PostgreSQL, and Docker.

> **Role:** Backend Engineer submission. Go backend only. Frontend is replaced by a Bruno collection under `api-collection/`.

---

## 1. Overview

**What this does**

- User registration and login with hashed passwords + JWT
- Create projects you own; list projects you own or have been assigned tasks in
- Create, filter, update, and delete tasks inside projects
- Enforce ownership rules (owner-only update/delete on projects; owner-or-creator on tasks)
- Aggregate project statistics by status and assignee

**Tech stack**

| Layer | Choice | Why |
|---|---|---|
| Language | Go 1.26 | Strong stdlib, great concurrency primitives, small binaries |
| Router | `go-chi/chi/v5` | Idiomatic `net/http` compatibility, no magic, explicit middleware composition |
| DB driver | `jackc/pgx/v5` | Modern native Postgres driver with proper types and a typed error surface (`pgconn.PgError`) |
| Migrations | `golang-migrate/migrate/v4` | Industry standard; explicit up + down SQL; auto-run on boot |
| Auth | `golang-jwt/jwt/v5` + `bcrypt` (cost 12) | Stateless JWT, 24h expiry; bcrypt hashes never leave the DB |
| Logging | `log/slog` (stdlib) | Structured JSON logs; zero dependency; Go 1.21+ standard |
| Container | Multi-stage Dockerfile + `docker compose` | Minimal runtime image on `alpine:3.21` |

---

## 2. Architecture Decisions

### Layered, not over-layered

```
cmd/api/main.go          → wiring, server lifecycle, graceful shutdown
internal/handler/        → HTTP handlers, validation, response shaping
internal/store/          → SQL queries, typed errors (ErrEmailTaken, ErrProjectNotFound, …)
internal/model/          → structs, enums, request validators
internal/middleware/     → auth, request logging
internal/database/       → pgx pool + migration + seed runner
internal/config/         → env loading with required-var checks
```

No service layer - at this scope it would be ceremony. Stores hold SQL, handlers hold HTTP concerns. If a third consumer of the logic appeared (CLI, scheduled job), a service layer would be warranted then.

### Raw SQL over ORM

Every query is hand-written and parameterized. Benefits:

- Every index and plan is visible
- No N+1 surprises from lazy loading
- Schema migrations own the source of truth, not struct tags
- `pgx`'s typed scans give compile-time column coverage

Trade-off: slightly more code per query. Worth it for a system where data access is the hot path.

### Auth boundary is the middleware, not the handler

`middleware.Auth` parses the `Authorization: Bearer …` header, validates the JWT signature and expiry, verifies `user_id` is non-empty, and injects it into `r.Context()`. Handlers call `middleware.GetUserID(ctx)` - they never see the raw token. This keeps token plumbing in one place.

### Errors: typed at the boundary

Store functions return `ErrUserNotFound`, `ErrEmailTaken`, `ErrProjectNotFound`, `ErrTaskNotFound`. Handlers `errors.Is()` them and map to the correct HTTP status. Unique-violation detection uses Postgres error code `23505` through `errors.As(err, &pgconn.PgError{})` - not string matching, which is fragile.

### Request correlation

Custom slog-based request logger (`middleware.RequestLogger`) attaches `request_id`, `method`, `path`, `status`, `duration_ms`, and `remote` to every log line. Paired with chi's `RequestID` middleware, every log in a request's lifecycle shares an ID.

### What I intentionally left out

| Skipped | Reason |
|---|---|
| Rate limiting | Not a take-home concern; easily added with `github.com/go-chi/httprate` as a middleware |
| Refresh tokens | 24h expiry + short-lived access token is sufficient at this scope |
| OpenAPI/Swagger spec | Bruno collection in `api-collection/` is the authoritative living doc |
| Repository interfaces | Handlers take concrete stores. Interfaces help mocking, but we ship a Bruno collection instead of unit tests - concrete types keep the code shorter |
| Custom error type hierarchy | Sentinel errors cover all current branches cleanly |
| Global tracing (OTel) | Would be the first thing I added for multi-service deployment |

---

## 3. Running Locally

Requires Docker and Docker Compose. Nothing else.

```bash
git clone <this repo>
cd taskflow-winson
docker compose up --build
```

That single command:

1. Pulls `postgres:16-alpine`
2. Builds the Go binary in a multi-stage image (`golang:1.26-alpine` → `alpine:3.21`)
3. Waits for Postgres to become healthy
4. Runs migrations (up)
5. Seeds the DB (one user, one project, three tasks)
6. Starts the API on **http://localhost:8080**

Verify it's up:

```bash
curl http://localhost:8080/health
# {"status":"ok"}

curl http://localhost:8080/health/ready
# {"status":"ready"}
```

### Using a .env file (optional)

Every env var has a sensible default in `docker-compose.yml`. For local overrides:

```bash
cp .env.example .env
# edit .env if desired
docker compose up --build
```

---

## 4. Running Migrations

Migrations run **automatically** at container start via `database.RunMigrations`. No manual steps.

Migrations are managed by [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate). Both `.up.sql` and `.down.sql` are committed for every migration under `backend/migrations/`.

To roll back manually (requires the `migrate` CLI on host):

```bash
docker compose exec api sh -c 'apk add --no-cache bash && wget -qO- https://github.com/golang-migrate/migrate/releases/latest/download/migrate.linux-amd64.tar.gz | tar xz && ./migrate -path /app/migrations -database "$DATABASE_URL" down 1'
```

For most work, the `up` migrations are sufficient and idempotent - bringing the schema back up from an empty DB.

---

## 5. Test Credentials

Seed data is loaded on first boot (`RUN_SEED=true` is the default in docker-compose).

```
Email:    test@example.com
Password: password123
```

You can turn off seeding with `RUN_SEED=false` - useful for production deployments.

---

## 6. API Reference

Base URL: `http://localhost:8080`

All endpoints return `application/json`. All non-auth endpoints require `Authorization: Bearer <token>`.

A ready-to-run Bruno collection is in [`api-collection/`](./api-collection). Open it with [Bruno](https://www.usebruno.com/) and run **auth/Login** once - the post-response script captures the token into a collection variable, and every other request uses it automatically.

### Health

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | - | Liveness probe |
| GET | `/health/ready` | - | Readiness probe (pings DB) |

### Auth

**POST `/auth/register`** - 201 Created

```json
// Request
{ "name": "Jane Doe", "email": "jane@example.com", "password": "secret123" }

// Response
{ "token": "<jwt>", "user": { "id": "uuid", "name": "Jane Doe", "email": "jane@example.com", "created_at": "..." } }
```

**POST `/auth/login`** - 200 OK

```json
// Request
{ "email": "test@example.com", "password": "password123" }

// Response
{ "token": "<jwt>", "user": { ... } }
```

### Projects

| Method | Path | Description |
|---|---|---|
| GET | `/projects?page=&limit=` | Paginated list of projects the user owns or has tasks in |
| POST | `/projects` | Create (owner = current user) |
| GET | `/projects/{id}` | Returns project with embedded `tasks` array |
| PATCH | `/projects/{id}` | Owner only |
| DELETE | `/projects/{id}` | Owner only; cascades to tasks |
| GET | `/projects/{id}/stats` | `{ total, by_status, by_assignee }` |

**GET `/projects?page=1&limit=50`**

```json
{
  "projects": [
    {
      "id": "uuid",
      "name": "Website Redesign",
      "description": "Q2 project",
      "owner_id": "uuid",
      "created_at": "2026-04-15T09:55:32Z"
    }
  ],
  "page": 1,
  "limit": 50
}
```

**GET `/projects/{id}/stats`**

```json
{
  "total": 3,
  "by_status": { "todo": 1, "in_progress": 1, "done": 1 },
  "by_assignee": [
    { "assignee_id": "uuid", "name": "Test User", "count": 2 },
    { "assignee_id": null, "name": "Unassigned", "count": 1 }
  ]
}
```

### Tasks

| Method | Path | Description |
|---|---|---|
| GET | `/projects/{id}/tasks?status=&assignee=&page=&limit=` | List with filters |
| POST | `/projects/{id}/tasks` | Create |
| PATCH | `/tasks/{id}` | Partial update (any field); supports optimistic locking via `expected_version` |
| DELETE | `/tasks/{id}` | Task creator or project owner |

**POST `/projects/{id}/tasks`**

```json
// Request
{ "title": "Design homepage", "description": "...", "priority": "high", "assignee_id": "uuid", "due_date": "2026-05-01" }

// Response - 201
{ "id": "uuid", "title": "...", "status": "todo", "priority": "high", ... }
```

`status` defaults to `todo`, `priority` defaults to `medium`. Valid values are enforced both by request validators and by PostgreSQL `ENUM` types.

### Error responses

```json
// 400 - validation
{ "error": "validation failed", "fields": { "email": "must be a valid email" } }

// 401 - missing/invalid token
{ "error": "unauthorized" }

// 403 - authenticated but not permitted
{ "error": "forbidden" }

// 404
{ "error": "not found" }

// 409 - optimistic locking conflict (task PATCH with stale expected_version)
{ "error": "task was modified by another request" }
```

---

## 7. What I'd Do With More Time

In priority order, the first few things I'd add for a real deployment:

1. **Rate limiting** on `/auth/*` - `go-chi/httprate` with a token bucket keyed by IP. Login brute-force protection is non-negotiable in production; I skipped it here to keep the demo self-contained.
2. **Refresh tokens** - rotating refresh tokens stored server-side with a `revoked_at`. The current 24h access token is fine for a demo but is not a production auth story.
3. **OpenAPI 3 spec** - generated from Go source with `swaggo/swag` or hand-maintained. Enables SDK generation and automated contract testing.
4. **Tracing & metrics** - OpenTelemetry with `otelhttp`/`otelpgx` middleware, export to Grafana Tempo / Prometheus. The current structured logs with `request_id` are a good half-step but aren't a trace.
5. **Broader test coverage** - the current 3 integration tests cover auth + project CRUD. A real codebase needs table-driven tests for every handler, plus `testcontainers-go` to isolate each test run with its own Postgres.
6. **Tighter CORS** - today `AllowedOrigins: ["*"]` for reviewer convenience; in prod this becomes an env-driven allowlist with `AllowCredentials: true` gated on the same origin list.
7. **Secrets via Vault / AWS Secrets Manager** - `JWT_SECRET` comes from env today; rotating keys with a JWKS endpoint is the real answer.
8. **Soft deletes + audit log** - `deleted_at` columns and an `audit_events` table for who-changed-what. Critical for any product shipped to customers.
9. **Background jobs** - task reminders, stats rollups. `river` or `asynq` both fit nicely with this stack.
10. **Pagination cursors** - offset/limit is fine for the demo but breaks on high-churn lists. A `created_at + id` cursor is the right long-term answer.

What I **wouldn't** add without a reason: a service layer, interface-based DI, a custom error framework, Kafka, Kubernetes manifests. YAGNI.

---

## 8. Project Layout

```
taskflow-winson/
├── backend/
│   ├── cmd/api/main.go                 # Entry point, wiring, graceful shutdown
│   ├── internal/
│   │   ├── config/config.go            # Env → Config with required-var checks
│   │   ├── database/db.go              # pgx pool + migration + seed runner
│   │   ├── handler/
│   │   │   ├── auth.go                 # Register, login
│   │   │   ├── health.go               # Liveness, readiness
│   │   │   ├── project.go              # Project CRUD
│   │   │   ├── task.go                 # Task CRUD + stats
│   │   │   ├── response.go             # JSON write helpers
│   │   │   └── handler_test.go         # Integration tests (auth + projects)
│   │   ├── middleware/
│   │   │   ├── auth.go                 # JWT middleware
│   │   │   └── logger.go               # slog-based request logger
│   │   ├── model/
│   │   │   ├── user.go
│   │   │   ├── project.go
│   │   │   └── task.go                 # Status + Priority enums
│   │   └── store/
│   │       ├── user.go                 # SQL + typed errors
│   │       ├── project.go
│   │       └── task.go
│   ├── migrations/
│   │   ├── 000001_init_schema.up.sql
│   │   └── 000001_init_schema.down.sql
│   ├── seed/seed.sql
│   ├── Dockerfile                      # Multi-stage build
│   └── go.mod
├── api-collection/                     # Bruno collection (reviewer-ready)
├── docker-compose.yml
├── .env.example
├── .gitignore
└── README.md
```
