package models

import "time"

// IdempotencyKey stores the outcome of a POST /transfers request keyed by
// (user_id, idempotency_key), so a retried request can be answered from the
// stored response instead of being re-applied.
type IdempotencyKey struct {
	ID             int64     `db:"id"`
	UserID         int64     `db:"user_id"`
	IdempotencyKey string    `db:"idempotency_key"`
	RequestHash    string    `db:"request_hash"`
	ResponseStatus *int      `db:"response_status"`
	ResponseBody   []byte    `db:"response_body"`
	CreatedAt      time.Time `db:"created_at"`
}
