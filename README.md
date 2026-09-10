# Mini Bank

A small internal funds-transfer API: register, log in, check your balance, transfer money to another account, and page through your transaction history. Built with Go, Gin, and Postgres to demonstrate clean architecture, safe concurrent balance updates, and idempotent transfers.

## Stack

- **Go** + **Gin** — HTTP layer
- **PostgreSQL** — storage, via `sqlx` and the `pgx` driver
- **JWT** — authentication
- **Docker / Docker Compose** — local run
- **Swagger (swaggo)** — API docs
- **OpenTelemetry** + **Jaeger** — distributed tracing (HTTP and DB spans)
- **log/slog** — structured JSON logging, correlated to traces

## Architecture

The code is organized in layers, each depending only on the one "inward" of it:

```
handler/       HTTP transport: routing, request/response DTOs, auth middleware
usecases/      Business logic. Defines the repository/infra interfaces it needs (ports.go)
repositories/  Postgres implementations of those interfaces (sqlx)
models/        Plain domain structs shared by repositories and usecases
pkg/           Infrastructure helpers: bcrypt hashing, JWT issuing/verification, logging, tracing
config/        Environment-based configuration
cmd/           main.go: wiring and graceful shutdown
```

The usecase layer owns the interfaces it depends on (`usecases/ports.go`) rather than importing concrete repository types — `repositories.AccountRepository` just happens to satisfy `usecases.AccountRepository` structurally. This keeps business logic testable without a database: `usecases/*_test.go` runs against in-memory fakes.

Multi-step writes (e.g. "create a user, then create their account") are composed with `TxManager.WithinTransaction`, which runs a closure inside one Postgres transaction. Repository calls made with the context passed into that closure automatically join the transaction — no repository needs to know whether it's in one.

## Data model

- `users` — id, name, email (unique, case-insensitive), password hash
- `accounts` — one per user, balance stored as a `BIGINT` (whole IDR, no subunit), `CHECK (balance >= 0)`
- `transfers` — one row per transfer, `CHECK (amount > 0)`, `CHECK (from_account_id <> to_account_id)`
- `entries` — an append-only ledger; every balance change writes one row here. This is both the audit trail and what backs the paginated history endpoint
- `idempotency_keys` — backs idempotent transfers, see below

See `migrations/001_initial_schema.sql`.

## Concurrency and correctness

- **Balance updates** lock both accounts with `SELECT ... FOR UPDATE` inside a single transaction, always in ascending account-ID order regardless of which account is the sender. This means two concurrent transfers between the same pair of accounts always request locks in the same order, which rules out a deadlock.
- **Idempotent transfers**: `POST /transfers` requires an `Idempotency-Key` header. The key (scoped per user) and a hash of the request body are recorded in the same transaction as the transfer itself. Retrying with the same key and body returns the original result without re-applying the transfer; reusing the key with a different body is rejected with `409 Conflict`.

## Observability

- **Logging** (`pkg/logging`): structured JSON on stdout via `log/slog`, level controlled by `LOG_LEVEL` (`debug`/`info`/`warn`/`error`, default `info`; `debug` also adds `source` file:line). Every request/error log line carries `service`/`version`/`env` and, whenever the request has an active span, `trace_id`/`span_id` — so a log line and its distributed trace can always be cross-referenced. A panic in a handler (`handler.Recovery`, replacing `gin.Recovery()`) is logged with its stack trace and turned into the same JSON error envelope every other failure returns, instead of a bare connection reset.
- **Tracing** (`pkg/telemetry`, OpenTelemetry): every HTTP request gets a span (`otelgin`), and every SQL statement executed within it gets a child span (`otelsql` wrapping the pgx driver) — so a trace shows exactly which queries a request ran and how long each took. Spans are always generated (so `trace_id` is always available for log correlation and is echoed back as the `X-Request-Id` response header); they're only shipped to a collector when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, so running without one configured is always safe. `docker compose up` points it at a bundled Jaeger instance — see below.

## Running it

```bash
cp .env.example .env   # then set a real JWT_SECRET
docker compose up --build
```

This starts Postgres, [Jaeger](https://www.jaegertracing.io/) (with `migrations/` mounted into `/docker-entrypoint-initdb.d`, so the schema applies automatically on first boot), and the API on `http://localhost:8080`.

API docs: `http://localhost:8080/docs` — try `POST /auth/login`, and its `access_token` is fed straight into the Authorize dialog, so the following calls (`GET /accounts/me/balance`, `POST /transfers`, ...) are authorized automatically with no copy/pasting. (The raw OpenAPI spec is also served at `/swagger/doc.json`, and swaggo's own UI at `/swagger/index.html`, without the auto-login behavior.)
Traces: `http://localhost:16686` (Jaeger UI) — pick service `mini-bank`.
Health check: `GET /healthz`

### Local development (without Docker)

```bash
cp .env.example .env
# point DB_HOST etc. at a Postgres instance and apply migrations/001_initial_schema.sql
make run
```

### Tests

```bash
make test
```

Usecase tests run against in-memory fakes, so no database is required.

### Regenerating Swagger docs

```bash
make swagger
```

## API

All responses are wrapped as `{"success": bool, "data": ..., "meta": ..., "error": {"code", "message"}}`. Authenticated endpoints require `Authorization: Bearer <token>`.

| Method | Path                      | Auth | Description                                  |
|--------|---------------------------|------|-----------------------------------------------|
| POST   | `/api/v1/auth/register`   | No   | Create a user + account prepopulated with 10,000,000 IDR |
| POST   | `/api/v1/auth/login`      | No   | Exchange credentials for a JWT                |
| GET    | `/api/v1/accounts/me/balance` | Yes | Current balance                          |
| POST   | `/api/v1/transfers`       | Yes  | Transfer funds (requires `Idempotency-Key` header) |
| GET    | `/api/v1/accounts/me/history` | Yes | Paginated ledger (`?page=&page_size=`) |

### Example

```bash
curl -X POST localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'

TOKEN=$(curl -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r .data.access_token)

curl -X POST localhost:8080/api/v1/transfers \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -H 'Idempotency-Key: <uuid>' \
  -d '{"to_account_id":2,"amount":1000000,"description":"rent"}'
```

## Design notes / assumptions

- One account per user (the spec's "transfer between 2 accounts" maps naturally to "transfer between 2 users' accounts"); transfers target a recipient by `to_account_id`.
- Balances are `BIGINT` whole-IDR amounts — no fractional subunit, matching how Rupiah is used in practice.
- Graceful shutdown: `cmd/main.go` listens for `SIGINT`/`SIGTERM`, stops accepting new connections, and lets in-flight requests finish (bounded by `SHUTDOWN_TIMEOUT`) before exiting.
