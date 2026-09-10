//go:build integration

// Package integration_test exercises the real HTTP router wired to a real
// Postgres (via testcontainers-go), unlike usecases/*_test.go which runs
// business logic against in-memory fakes. It's the only place that proves
// the SELECT ... FOR UPDATE locking and idempotency-key logic actually hold
// up against real transactions and real concurrency — build-tagged out of
// `go test ./...` because it needs a Docker daemon and is much slower.
//
// Run with: make test-integration (or `go test -tags=integration ./test/integration/...`).
package integration_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rozy97/mini-bank/app"
	"github.com/rozy97/mini-bank/config"
)

var (
	testServer *httptest.Server
	testDB     *sqlx.DB
)

func TestMain(m *testing.M) {
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

// run does the container/server setup and teardown around m.Run(). It must
// return rather than os.Exit so its defers actually execute — os.Exit skips
// deferred calls, which is the classic TestMain footgun.
func run(m *testing.M) (int, error) {
	ctx := context.Background()

	migrationPath, err := filepath.Abs(filepath.Join("..", "..", "migrations", "001_initial_schema.sql"))
	if err != nil {
		return 1, fmt.Errorf("resolve migration path: %w", err)
	}

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("minibank_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts(migrationPath),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return 1, fmt.Errorf("start postgres container: %w", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 1, fmt.Errorf("get connection string: %w", err)
	}

	testDB, err = sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return 1, fmt.Errorf("connect to test database: %w", err)
	}
	defer testDB.Close()

	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "integration-test-secret", TTL: time.Hour},
	}

	testServer = httptest.NewServer(app.NewRouter(testDB, cfg))
	defer testServer.Close()

	return m.Run(), nil
}
