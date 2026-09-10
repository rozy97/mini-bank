// Package app wires the application's layers together into an
// http.Handler. It exists so cmd/main.go and the integration tests build
// the exact same object graph instead of two independent (and potentially
// diverging) copies of the wiring.
package app

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/rozy97/mini-bank/config"
	"github.com/rozy97/mini-bank/handler"
	"github.com/rozy97/mini-bank/pkg/password"
	"github.com/rozy97/mini-bank/pkg/token"
	"github.com/rozy97/mini-bank/repositories"
	"github.com/rozy97/mini-bank/usecases"
)

// NewRouter builds the full dependency graph — repositories, usecases,
// handlers — on top of db and returns the resulting HTTP router.
func NewRouter(db *sqlx.DB, cfg *config.Config) *gin.Engine {
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
	}, tokenManager, cfg.OTel.ServiceName)
}
