// Mini Bank API.
//
// @title        Mini Bank API
// @version      1.0
// @description  Internal funds transfer API demonstrating clean architecture with Gin, Postgres, and idempotent transfers.
// @BasePath     /api/v1
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                Type "Bearer" followed by a space and the JWT returned by /auth/login.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rozy97/mini-bank/app"
	"github.com/rozy97/mini-bank/config"
	_ "github.com/rozy97/mini-bank/docs"
	"github.com/rozy97/mini-bank/pkg/logging"
	"github.com/rozy97/mini-bank/pkg/telemetry"
)

const version = "1.0.0"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(logging.New(os.Stdout, logging.ParseLevel(cfg.LogLevel),
		slog.String("service", cfg.OTel.ServiceName),
		slog.String("version", version),
		slog.String("env", cfg.AppEnv),
	))

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx := context.Background()

	shutdownTracing, err := telemetry.InitTracerProvider(ctx, telemetry.Config{
		ServiceName:      cfg.OTel.ServiceName,
		ServiceVersion:   version,
		Environment:      cfg.AppEnv,
		ExporterEndpoint: cfg.OTel.ExporterEndpoint,
		Insecure:         cfg.OTel.Insecure,
	})
	if err != nil {
		slog.Error("failed to initialize tracing", "error", err)
		os.Exit(1)
	}

	// otelsql wraps the pgx driver so every query executed through db emits
	// a child span (visible under the HTTP request span that triggered it).
	sqlDB, err := otelsql.Open("pgx", cfg.DB.DSN(), otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL))
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	db := sqlx.NewDb(sqlDB, "pgx")
	defer db.Close()

	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	router := app.NewRouter(db, cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-stopCtx.Done()
	stop()
	slog.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	if err := shutdownTracing(shutdownCtx); err != nil {
		slog.Error("failed to shut down tracing", "error", err)
	}

	slog.Info("server exited gracefully")
}
