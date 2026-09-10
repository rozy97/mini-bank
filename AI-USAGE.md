# AI Usage

This document explains how AI was used to build Mini Bank, so anyone reviewing the codebase or its history knows what to expect and how much scrutiny to apply.

## Tool and model

The entire application — from the initial scaffold onward — was built with **[Claude Code](https://claude.com/claude-code)** running **Claude Sonnet 5**, working directly in the project's terminal with full access to read/write files, run shell commands, use a browser, and manage git. Every commit on this branch after the human-authored scaffold (see [Starting point](#starting-point)) carries:

```
Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_...
```

so the exact commit-by-commit provenance and the full session transcript behind each change are traceable, not just asserted here.

## Starting point

The human author scaffolded the repository before AI was involved: `go.mod`, an empty `main.go`, a placeholder `models.User` / `repositories.Repository`, a first pass at `migrations/001_initial_schema.sql` and `migrations/002_seed.sql`, a GitHub Actions CI workflow running `go test ./...`, and a couple of trivial placeholder tests (commits `initial commit` through `test: add failing test case`). The human then gave AI a single natural-language brief: implement the full application — clean architecture, Gin, Postgres, Docker Compose, Swagger, graceful shutdown, and the reviewed/improved schema — with all five APIs (register with prepopulated balance, login, get balance, idempotent transfer, paginated history).

## What AI did

Essentially all of the application code, infrastructure config, tests, and documentation now in this repository was authored by Claude Code:

- **Application code** — the full clean-architecture implementation (`models/`, `repositories/`, `usecases/`, `handler/`, `pkg/`, `config/`, `app/`, `cmd/main.go`), including the domain design decisions (one account per user, `BIGINT` IDR balances, the append-only `entries` ledger, the `idempotency_keys` table and its request-hash conflict detection, the ascending-account-ID lock ordering that rules out deadlocks on concurrent transfers).
- **Schema** — reviewed and rewrote the original `initial_schema.sql` (case-insensitive unique email, FK/CHECK constraints, the ledger and idempotency tables, `updated_at` triggers).
- **Tests** — the in-memory-fake-backed unit tests in `usecases/*_test.go` (99%+ coverage, including every error branch and both lock-ordering paths), and the `testcontainers-go`-based integration suite in `test/integration/` that runs the real HTTP stack against a real Postgres, including a concurrency test that fires dozens of transfers at the same two accounts simultaneously and asserts the total balance is conserved.
- **Infrastructure** — the multi-stage `Dockerfile`, `docker-compose.yml` (Postgres, Jaeger, the app, nginx), `nginx/default.conf` (security headers, gzip, rate limiting), the `Makefile`, the GitHub Actions CI jobs, and the OpenTelemetry/structured-logging wiring (`pkg/telemetry`, `pkg/logging`).
- **Documentation** — this file, `README.md`, and the Swagger annotations that generate `docs/`.

## How it was used — process, not just output

This wasn't "generate once and ship." The build happened as an iterative, conversational pair-programming session, with the human directing scope and priorities turn by turn: implement the app → split the resulting work into logically-separated commits → run and manually test it → add automatic Swagger login (prompted by the human noticing the manual token copy-paste) → maximize unit test coverage → add production-grade logging and tracing → add nginx → add integration tests → rewrite the README as a practical how-to. Each of those was a distinct request the human made after seeing the result of the last one, not a single unsupervised generation pass.

Within that process, AI's own verification discipline included:

- **Running the actual test suites** (`go test ./... -race`, and separately the integration suite against a real Postgres) after essentially every change, not just writing tests and assuming they'd pass.
- **Standing up the real Docker Compose stack** and exercising it with `curl` for every endpoint and error path (duplicate email, insufficient balance, wrong idempotency key, unauthorized access, concurrent transfers) rather than reasoning about behavior in the abstract.
- **Using a real browser** (via a Claude-in-Chrome integration) to click through the custom Swagger docs page and confirm the auto-authorize-after-login behavior actually worked end to end, instead of trusting the JavaScript by inspection alone.
- **Querying Jaeger's own API** after generating traffic to confirm a transfer request actually produced one root HTTP span with correctly nested child database spans, rather than assuming the OpenTelemetry wiring was correct because it compiled.
- **Catching and fixing a real bug** discovered through manual testing: the auth middleware originally did a case-sensitive `Bearer` prefix match, which rejected the lowercase `bearer` scheme curl and some tools send by default — found when the human tried a `curl` command against the running app, fixed, and re-verified live.

## Human oversight

The human directed every phase of the work (what to build, in what order, and several mid-course corrections — e.g. asking for the initial single commit to be split into logically-separated commits after the fact, which required rewriting and force-pushing branch history), reviewed the running application directly by exercising it themselves between AI-driven sessions, and is the one merging/pushing every commit to the shared `develop` branch. AI did not have — and does not have — the ability to merge to `main`, deploy anywhere, or make anything publicly reachable on its own; every push seen in this repository's history was made by AI code committed and pushed at the human's direction, to a branch the human controls.

## Caveats for reviewers

- This codebase has not had a separate, independent human code review pass beyond the human's own testing and direction described above. Treat it accordingly if adopting it for anything beyond its stated purpose (a demonstration project).
- AI self-reported its own testing and verification throughout the session (as summarized above); those claims are traceable in the linked session transcript but, like any changelog, are worth spot-checking rather than taken purely on faith.
- Secrets in this repository (`.env`, JWT secrets) are local-development placeholders only, excluded from version control via `.gitignore`; they were never real credentials and nothing sensitive was ever exposed to or generated by AI.
