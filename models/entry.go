package models

import "time"

// Entry is a single append-only ledger line for an account. A transfer
// produces exactly two entries: a debit on the source account and a credit
// on the destination account.
type Entry struct {
	ID           int64     `db:"id"`
	AccountID    int64     `db:"account_id"`
	TransferID   *int64    `db:"transfer_id"`
	Amount       int64     `db:"amount"`
	BalanceAfter int64     `db:"balance_after"`
	CreatedAt    time.Time `db:"created_at"`
}
