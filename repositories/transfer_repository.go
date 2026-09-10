package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rozy97/mini-bank/models"
)

type TransferRepository struct {
	db *sqlx.DB
}

func NewTransferRepository(db *sqlx.DB) *TransferRepository {
	return &TransferRepository{db: db}
}

func (r *TransferRepository) Create(ctx context.Context, t *models.Transfer) error {
	ex := executorFromContext(ctx, r.db)
	query := `
		INSERT INTO transfers (from_account_id, to_account_id, amount, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	if err := ex.GetContext(ctx, t, query, t.FromAccountID, t.ToAccountID, t.Amount, t.Description); err != nil {
		return fmt.Errorf("insert transfer: %w", err)
	}
	return nil
}
