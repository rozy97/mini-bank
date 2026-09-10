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

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rozy97/mini-bank/config"
	_ "github.com/rozy97/mini-bank/docs"
	"github.com/rozy97/mini-bank/handler"
	"github.com/rozy97/mini-bank/pkg/password"
	"github.com/rozy97/mini-bank/pkg/token"
	"github.com/rozy97/mini-bank/repositories"
	"github.com/rozy97/mini-bank/usecases"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := sqlx.Connect("pgx", cfg.DB.DSN())
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)

	router := buildRouter(db, cfg)

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-ctx.Done()
	stop()
	slog.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}

func buildRouter(db *sqlx.DB, cfg *config.Config) *gin.Engine {
	txManager := repositories.NewTxManager(db)
	userRepo := repositories.NewUserRepository(db)
	accountRepo := repositories.NewAccountRepository(db)
	transferRepo := repositories.NewTransferRepository(db)
	entryRepo := repositories.NewEntryRepository(db)
	idempotencyRepo := repositories.NewIdempotencyRepository(db)

	hasher := password.NewBcryptHasher()
	tokenManager := token.NewJWTManager(cfg.JWT.Secret, cfg.JWT.TTL)

	authUC := usecases.NewAuthUsecase(userRepo, accountRepo, txManager, hasher, tokenManager)
	accountUC := usecases.NewAccountUsecase(accountRepo, entryRepo)
	transferUC := usecases.NewTransferUsecase(accountRepo, transferRepo, entryRepo, idempotencyRepo, txManager)

	return handler.NewRouter(handler.Handlers{
		Auth:     handler.NewAuthHandler(authUC),
		Account:  handler.NewAccountHandler(accountUC),
		Transfer: handler.NewTransferHandler(transferUC),
	}, tokenManager)
}
