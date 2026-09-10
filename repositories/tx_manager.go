package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// TxManager runs a function inside a single database transaction. Repository
// calls made with the context it hands to fn automatically participate in
// that transaction (see executorFromContext), so usecases can compose
// several repository calls into one atomic unit of work without any
// repository being aware of transactions itself.
type TxManager struct {
	db *sqlx.DB
}

func NewTxManager(db *sqlx.DB) *TxManager {
	return &TxManager{db: db}
}

func (tm *TxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		// Already inside a transaction (nested call): reuse it rather than
		// starting a new one, since Postgres doesn't support nested txns.
		return fn(ctx)
	}

	tx, err := tm.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	err = fn(context.WithValue(ctx, txKey{}, tx))
	return err
}
