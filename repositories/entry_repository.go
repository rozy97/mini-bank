package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rozy97/mini-bank/models"
)

type EntryRepository struct {
	db *sqlx.DB
}

func NewEntryRepository(db *sqlx.DB) *EntryRepository {
	return &EntryRepository{db: db}
}

func (r *EntryRepository) Create(ctx context.Context, e *models.Entry) error {
	ex := executorFromContext(ctx, r.db)
	query := `
		INSERT INTO entries (account_id, transfer_id, amount, balance_after)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	if err := ex.GetContext(ctx, e, query, e.AccountID, e.TransferID, e.Amount, e.BalanceAfter); err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}
	return nil
}

// ListByAccountID returns entries newest-first.
func (r *EntryRepository) ListByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]models.Entry, error) {
	ex := executorFromContext(ctx, r.db)
	var entries []models.Entry
	query := `
		SELECT id, account_id, transfer_id, amount, balance_after, created_at
		FROM entries
		WHERE account_id = $1
		ORDER BY id DESC
		LIMIT $2 OFFSET $3
	`
	if err := ex.SelectContext(ctx, &entries, query, accountID, limit, offset); err != nil {
		return nil, fmt.Errorf("list entries by account id: %w", err)
	}
	return entries, nil
}

func (r *EntryRepository) CountByAccountID(ctx context.Context, accountID int64) (int64, error) {
	ex := executorFromContext(ctx, r.db)
	var count int64
	query := `SELECT COUNT(*) FROM entries WHERE account_id = $1`
	if err := ex.GetContext(ctx, &count, query, accountID); err != nil {
		return 0, fmt.Errorf("count entries by account id: %w", err)
	}
	return count, nil
}
