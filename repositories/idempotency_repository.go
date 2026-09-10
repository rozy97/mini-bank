package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rozy97/mini-bank/models"
)

type IdempotencyRepository struct {
	db *sqlx.DB
}

func NewIdempotencyRepository(db *sqlx.DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

// FindOrCreate atomically reserves (userID, key) for this caller. When the
// key is new it returns the freshly inserted (empty) record and created =
// true. When the key already exists it returns the existing record —
// including any previously stored response — and created = false. Must run
// inside a transaction so a concurrent retry of the same key blocks on the
// row rather than racing this call.
func (r *IdempotencyRepository) FindOrCreate(ctx context.Context, userID int64, key, requestHash string) (*models.IdempotencyKey, bool, error) {
	ex := executorFromContext(ctx, r.db)

	var rec models.IdempotencyKey
	insertQuery := `
		INSERT INTO idempotency_keys (user_id, idempotency_key, request_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, idempotency_key) DO NOTHING
		RETURNING id, user_id, idempotency_key, request_hash, response_status, response_body, created_at
	`
	err := ex.GetContext(ctx, &rec, insertQuery, userID, key, requestHash)
	if err == nil {
		return &rec, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("insert idempotency key: %w", err)
	}

	selectQuery := `
		SELECT id, user_id, idempotency_key, request_hash, response_status, response_body, created_at
		FROM idempotency_keys
		WHERE user_id = $1 AND idempotency_key = $2
	`
	if err := ex.GetContext(ctx, &rec, selectQuery, userID, key); err != nil {
		return nil, false, fmt.Errorf("select idempotency key: %w", err)
	}
	return &rec, false, nil
}

func (r *IdempotencyRepository) Complete(ctx context.Context, id int64, status int, body []byte) error {
	ex := executorFromContext(ctx, r.db)
	query := `UPDATE idempotency_keys SET response_status = $1, response_body = $2 WHERE id = $3`
	if _, err := ex.ExecContext(ctx, query, status, body, id); err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}
