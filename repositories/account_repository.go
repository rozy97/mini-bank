package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rozy97/mini-bank/models"
)

type AccountRepository struct {
	db *sqlx.DB
}

func NewAccountRepository(db *sqlx.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) Create(ctx context.Context, a *models.Account) error {
	ex := executorFromContext(ctx, r.db)
	query := `
		INSERT INTO accounts (owner_id, balance, currency)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`
	if err := ex.GetContext(ctx, a, query, a.OwnerID, a.Balance, a.Currency); err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// GetByOwnerID returns sql.ErrNoRows (wrapped) when the user has no account.
func (r *AccountRepository) GetByOwnerID(ctx context.Context, ownerID int64) (*models.Account, error) {
	ex := executorFromContext(ctx, r.db)
	var a models.Account
	query := `
		SELECT id, owner_id, balance, currency, created_at, updated_at
		FROM accounts
		WHERE owner_id = $1
	`
	if err := ex.GetContext(ctx, &a, query, ownerID); err != nil {
		return nil, fmt.Errorf("get account by owner id: %w", err)
	}
	return &a, nil
}

// GetForUpdate locks the account row for the lifetime of the enclosing
// transaction. It must only be called inside TxManager.WithinTransaction.
func (r *AccountRepository) GetForUpdate(ctx context.Context, id int64) (*models.Account, error) {
	ex := executorFromContext(ctx, r.db)
	var a models.Account
	query := `
		SELECT id, owner_id, balance, currency, created_at, updated_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`
	if err := ex.GetContext(ctx, &a, query, id); err != nil {
		return nil, fmt.Errorf("get account for update: %w", err)
	}
	return &a, nil
}

func (r *AccountRepository) UpdateBalance(ctx context.Context, id int64, balance int64) error {
	ex := executorFromContext(ctx, r.db)
	query := `UPDATE accounts SET balance = $1 WHERE id = $2`
	if _, err := ex.ExecContext(ctx, query, balance, id); err != nil {
		return fmt.Errorf("update account balance: %w", err)
	}
	return nil
}
