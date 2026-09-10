package repositories

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// executor is the subset of *sqlx.DB / *sqlx.Tx that repositories need.
// It lets every repository method run unmodified whether or not it is
// currently inside a transaction started by TxManager.
type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
}

type txKey struct{}

// executorFromContext returns the *sqlx.Tx stashed in ctx by TxManager, or
// falls back to db when the caller is not inside a transaction.
func executorFromContext(ctx context.Context, db *sqlx.DB) executor {
	if tx, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return tx
	}
	return db
}
