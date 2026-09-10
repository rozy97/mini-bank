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
- **nginx** — reverse proxy in front of the API

## Quickstart

```bash
cp .env.example .env   # then set a real JWT_SECRET
docker compose up --build
```

That's it — Postgres, Jaeger, the API, and nginx all start together, with the schema applied automatically. Jump to [Services](#services) for where everything lives, or [API reference](#api-reference) to start calling it.

## Services

`docker compose up` starts everything below. Everything under **URL** is reachable from your host machine once the stack is up.

| Service | Container | URL | What it's for |
|---|---|---|---|
| nginx (front door) | `nginx` | `http://localhost` | Reverse-proxied API — the intended entry point |
| API (direct) | `app` | `http://localhost:8080` | Same API, bypassing nginx — handy for local debugging |
| Interactive API docs | `app` | `http://localhost/docs` | Swagger UI; logging in via `POST /auth/login` here auto-fills the Authorize dialog |
| Swagger UI (default) | `app` | `http://localhost/swagger/index.html` | swaggo's stock UI, no auto-login |
| OpenAPI spec (raw) | `app` | `http://localhost/swagger/doc.json` | The spec itself, e.g. for codegen |
| Health check | `app` | `http://localhost/healthz` | Liveness probe; excluded from logs/traces to cut noise |
| Jaeger UI (traces) | `jaeger` | `http://localhost:16686` | Pick service `mini-bank` to see request traces with nested DB spans |
| Postgres | `postgres` | `localhost:5432` | `psql -h localhost -U postgres` (password `postgres` by default, see `.env`) |

### Running without Docker

```bash
cp .env.example .env
# point DB_HOST etc. at a Postgres instance and apply migrations/001_initial_schema.sql
make run
```

The API listens on `:8080` (or `$PORT`); there's no nginx/Jaeger in front of it in this mode, and tracing stays local-only (see [Observability](#observability)) unless you point `OTEL_EXPORTER_OTLP_ENDPOINT` at a collector yourself.

## API reference

All responses are wrapped as `{"success": bool, "data": ..., "meta": ..., "error": {"code", "message"}}`. Authenticated endpoints require `Authorization: Bearer <token>` (case-insensitive scheme, so `bearer <token>` works too).

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/register` | No | Create a user + account prepopulated with 10,000,000 IDR |
| POST | `/api/v1/auth/login` | No | Exchange credentials for a JWT |
| GET | `/api/v1/accounts/me/balance` | Yes | Current balance |
| GET | `/api/v1/accounts/me/history` | Yes | Paginated ledger (`?page=&page_size=`, default 1/20, max page_size 100) |
| POST | `/api/v1/transfers` | Yes | Transfer funds (requires `Idempotency-Key` header) |
| GET | `/healthz` | No | Health check |

The examples below go through nginx on port 80; swap `localhost` for `localhost:8080` to hit the API directly.

### Register

```bash
curl -X POST localhost/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
```

```json
{"success":true,"data":{"user_id":1,"name":"Alice","email":"alice@example.com","account_id":1,"balance":10000000,"currency":"IDR"}}
```

### Login

```bash
curl -X POST localhost/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'
```

```json
{"success":true,"data":{"access_token":"eyJhbGciOiJIUzI1NiIs...","token_type":"Bearer","expires_at":"2026-09-11T07:26:34Z"}}
```

Save the token for the requests below:

```bash
TOKEN=$(curl -s -X POST localhost/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r .data.access_token)
```

### Get balance

```bash
curl localhost/api/v1/accounts/me/balance -H "Authorization: Bearer $TOKEN"
```

```json
{"success":true,"data":{"account_id":1,"balance":10000000,"currency":"IDR"}}
```

### Transfer funds

`Idempotency-Key` is required — any client-generated string unique per transfer *attempt*. Retrying the same key with the same body is safe (returns the original result instead of transferring twice); reusing it with a different body is rejected with `409`.

```bash
curl -X POST localhost/api/v1/transfers \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 3f29b6c1-6b8e-4e41-9c2a-2a2f9e6b6b41' \
  -d '{"to_account_id":2,"amount":1000000,"description":"rent"}'
```

```json
{"success":true,"data":{"transfer_id":1,"from_account_id":1,"to_account_id":2,"amount":1000000,"description":"rent","created_at":"2026-09-10T07:26:34.935996Z"}}
```

Retrying it (same key, same body) returns the same `transfer_id` and doesn't move money again:

```bash
curl -X POST localhost/api/v1/transfers \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 3f29b6c1-6b8e-4e41-9c2a-2a2f9e6b6b41' \
  -d '{"to_account_id":2,"amount":1000000,"description":"rent"}'
```

Error responses you'll actually hit while testing:

| Situation | Status | `error.code` |
|---|---|---|
| Missing/invalid bearer token | 401 | `UNAUTHORIZED` |
| Duplicate email on register | 409 | `EMAIL_ALREADY_EXISTS` |
| Wrong password / unknown email on login | 401 | `INVALID_CREDENTIALS` |
| Recipient account doesn't exist | 404 | `ACCOUNT_NOT_FOUND` |
| Amount exceeds balance | 422 | `INSUFFICIENT_BALANCE` |
| `to_account_id` equals your own account, amount ≤ 0, or missing `Idempotency-Key` | 400 | `INVALID_REQUEST` |
| Idempotency key reused with a different body, or a concurrent attempt with the same key is still in flight | 409 | `IDEMPOTENCY_KEY_CONFLICT` |

### Get paginated history

```bash
curl "localhost/api/v1/accounts/me/history?page=1&page_size=20" -H "Authorization: Bearer $TOKEN"
```

```json
{"success":true,"data":[{"id":1,"transfer_id":1,"amount":-1000000,"balance_after":9000000,"created_at":"2026-09-10T07:26:34.935996Z"}],"meta":{"page":1,"page_size":20,"total_items":1,"total_pages":1}}
```

Entries are newest-first; `amount` is negative for a debit (money leaving this account) and positive for a credit.

## Environment variables

All read by `config.Load()` (`config/config.go`); see `.env.example` for a ready-to-copy file. Everything has a sensible default except `JWT_SECRET`, which is required.

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | `production` switches Gin to release mode |
| `PORT` | `8080` | |
| `SHUTDOWN_TIMEOUT` | `10s` | How long graceful shutdown waits for in-flight requests |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error`; `debug` also adds `source` file:line |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | `localhost`, `5432`, `postgres`, `postgres`, `postgres`, `disable` | |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS` | `25`, `25` | |
| `DB_CONN_MAX_LIFETIME` | `5m` | |
| `JWT_SECRET` | *(none — required)* | |
| `JWT_TTL` | `24h` | |
| `OTEL_SERVICE_NAME` | `mini-bank` | |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(empty)* | `host:port` of an OTLP/HTTP collector, e.g. `jaeger:4318`. Empty disables export (spans are still generated, just not shipped anywhere) |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Plaintext (no TLS) to the collector |

## Architecture

The code is organized in layers, each depending only on the one "inward" of it:

```
handler/       HTTP transport: routing, request/response DTOs, auth middleware
usecases/      Business logic. Defines the repository/infra interfaces it needs (ports.go)
repositories/  Postgres implementations of those interfaces (sqlx)
models/        Plain domain structs shared by repositories and usecases
pkg/           Infrastructure helpers: bcrypt hashing, JWT issuing/verification, logging, tracing
config/        Environment-based configuration
app/           Builds the full dependency graph into an http.Handler; shared by cmd/main.go and the integration tests
cmd/           main.go: process wiring and graceful shutdown
test/          test/integration: full-stack tests against a real Postgres (see Testing below)
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

- **Balance updates** lock both accounts with `SELECT ... FOR UPDATE` inside a single transaction, always in ascending account-ID order regardless of which account is the sender. This means two concurrent transfers between the same pair of accounts always request locks in the same order, which rules out a deadlock. `test/integration/concurrency_test.go` proves this against a real database under real concurrency, not just in theory.
- **Idempotent transfers**: `POST /transfers` requires an `Idempotency-Key` header. The key (scoped per user) and a hash of the request body are recorded in the same transaction as the transfer itself. Retrying with the same key and body returns the original result without re-applying the transfer; reusing the key with a different body is rejected with `409 Conflict`.

## Observability

- **Logging** (`pkg/logging`): structured JSON on stdout via `log/slog`, level controlled by `LOG_LEVEL` (`debug`/`info`/`warn`/`error`, default `info`; `debug` also adds `source` file:line). Every request/error log line carries `service`/`version`/`env` and, whenever the request has an active span, `trace_id`/`span_id` — so a log line and its distributed trace can always be cross-referenced. A panic in a handler (`handler.Recovery`, replacing `gin.Recovery()`) is logged with its stack trace and turned into the same JSON error envelope every other failure returns, instead of a bare connection reset.
- **Tracing** (`pkg/telemetry`, OpenTelemetry): every HTTP request gets a span (`otelgin`), and every SQL statement executed within it gets a child span (`otelsql` wrapping the pgx driver) — so a trace shows exactly which queries a request ran and how long each took. Spans are always generated (so `trace_id` is always available for log correlation and is echoed back as the `X-Request-Id` response header); they're only shipped to a collector when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, so running without one configured is always safe. `docker compose up` points it at a bundled Jaeger instance.

## Reverse proxy

`nginx` (`nginx/default.conf`) sits in front of the API on port 80: it sets baseline security headers, gzips JSON responses, rate-limits the unauthenticated `/api/v1/auth/*` endpoints (5 req/s per IP, burst 10, `429` once exceeded), and forwards `X-Real-IP`/`X-Forwarded-For`/`X-Forwarded-Proto` so the app logs the real client IP. Gin is configured to trust `X-Forwarded-For` only from private-network peers (`handler.NewRouter`'s `SetTrustedProxies` call) — i.e. nginx itself, not whatever a public client claims. The app's own port stays published too (`:8080`) for direct local debugging; nginx (`:80`) is the intended front door.

## Testing

```bash
make test              # unit tests: usecases against in-memory fakes, no database needed
make test-integration  # integration tests: real HTTP + real Postgres (needs Docker)
```

**Unit tests** (`usecases/*_test.go`, 99%+ coverage) exercise business logic — registration, login, balance, transfers, pagination, and every error branch — against hand-written in-memory fakes of the repository/token/hasher interfaces. Fast, no external services.

**Integration tests** (`test/integration/`, build-tagged `integration` so `go test ./...` skips them) spin up Postgres via [testcontainers-go](https://golang.testcontainers.org/), apply the real `migrations/001_initial_schema.sql`, and drive the real router — `app.NewRouter`, the same constructor `cmd/main.go` uses — over real HTTP. This is what the fake-backed unit tests structurally can't prove: whether the `SELECT ... FOR UPDATE` locking and idempotency-key logic actually hold up against a real database and real concurrency:

- `auth_test.go`, `transfer_test.go` — the full request/response contract, including the error matrix above, against a real database
- `concurrency_test.go` — fires dozens of transfers back and forth between the same two accounts concurrently and asserts the total balance is exactly conserved with nothing negative or deadlocked; fires the same idempotency key concurrently and asserts every response reports the same transfer

Both `make test` and `make test-integration` run in CI (`.github/workflows/go-test.yml`) as separate jobs.

### Regenerating Swagger docs

```bash
make swagger
```

Re-scans the `@...` annotations in `cmd/main.go` and `handler/*.go` and regenerates `docs/`.

## Design notes / assumptions

- One account per user (the spec's "transfer between 2 accounts" maps naturally to "transfer between 2 users' accounts"); transfers target a recipient by `to_account_id`.
- Balances are `BIGINT` whole-IDR amounts — no fractional subunit, matching how Rupiah is used in practice.
- Graceful shutdown: `cmd/main.go` listens for `SIGINT`/`SIGTERM`, stops accepting new connections, and lets in-flight requests finish (bounded by `SHUTDOWN_TIMEOUT`) before exiting.
