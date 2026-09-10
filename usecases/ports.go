package usecases

import (
	"context"
	"time"

	"github.com/rozy97/mini-bank/models"
)

// The interfaces below are the ports this layer depends on. They are
// defined here, next to the business logic that needs them, and satisfied
// structurally by the repositories/ and pkg/ packages — the usecase layer
// never imports those packages' concrete types.

type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
}

type AccountRepository interface {
	Create(ctx context.Context, a *models.Account) error
	GetByOwnerID(ctx context.Context, ownerID int64) (*models.Account, error)
	GetForUpdate(ctx context.Context, id int64) (*models.Account, error)
	UpdateBalance(ctx context.Context, id int64, balance int64) error
}

type TransferRepository interface {
	Create(ctx context.Context, t *models.Transfer) error
}

type EntryRepository interface {
	Create(ctx context.Context, e *models.Entry) error
	ListByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]models.Entry, error)
	CountByAccountID(ctx context.Context, accountID int64) (int64, error)
}

type IdempotencyRepository interface {
	FindOrCreate(ctx context.Context, userID int64, key, requestHash string) (*models.IdempotencyKey, bool, error)
	Complete(ctx context.Context, id int64, status int, body []byte) error
}

// TxManager runs fn atomically; repository calls made through the ctx it
// passes to fn participate in the same transaction.
type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenManager interface {
	Generate(userID int64, email string) (token string, expiresAt time.Time, err error)
}
