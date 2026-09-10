package usecases

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolationCode = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolationCode
	}
	return false
}
